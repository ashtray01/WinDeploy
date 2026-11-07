package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"windeploy/core/deploy"
	"windeploy/core/engine"
	"windeploy/core/inventory"
)

// Stops preparation before running/wg registration, exposing the former race.
type preparationProvider struct {
	fixtureProvider
	once    sync.Once
	entered chan struct{}
	release chan struct{}
}

func (p *preparationProvider) System(context.Context) (inventory.SystemInfo, error) {
	p.once.Do(func() { close(p.entered) })
	<-p.release
	return inventory.SystemInfo{Architecture: "amd64", Windows: inventory.Version{Major: 10, Build: 26100, ProductType: 1}}, nil
}

func awaitSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("barrier timeout")
	}
}

func pendingSession(t *testing.T, s *Service) string {
	t.Helper()
	tasks := []engine.Task{{ID: "dns", Name: "DNS", Category: "diagnostics", Status: engine.Pending, Parameters: map[string]string{"dns": "localhost", "dhcp": "false", "gateways": "false"}}}
	snapshot, err := engine.NewSnapshot(tasks, nil)
	if err != nil {
		t.Fatal(err)
	}
	session := deploy.Session{Version: deploy.SessionVersion, ID: deploy.NewSessionID(), ProfileID: "selftest", Tasks: tasks, Snapshot: snapshot}
	if err := s.deployment.store.Save(context.Background(), session); err != nil {
		t.Fatal(err)
	}
	return session.ID
}

func TestDeploymentPreparationExcludesOtherWriters(t *testing.T) {
	for _, resume := range []bool{false, true} {
		t.Run(map[bool]string{false: "start", true: "resume"}[resume], func(t *testing.T) {
			s := newDeploymentTestService(t)
			id := pendingSession(t, s)
			provider := &preparationProvider{entered: make(chan struct{}), release: make(chan struct{})}
			s.provider = provider
			var unblock sync.Once
			defer unblock.Do(func() { close(provider.release) })
			first := make(chan error, 1)
			go func() {
				var err error
				if resume {
					_, err = s.ResumeDeployment(id, false)
				} else {
					_, err = s.StartDeployment("selftest", nil)
				}
				first <- err
			}()
			awaitSignal(t, provider.entered)
			var wg sync.WaitGroup
			errs := make(chan error, 3)
			for _, request := range []func() error{
				func() error { _, err := s.StartPackageInstall([]string{"sevenzip"}); return err },
				func() error { _, err := s.StartDeployment("selftest", nil); return err },
				func() error { _, err := s.ResumeDeployment(id, false); return err },
			} {
				wg.Add(1)
				go func(request func() error) { defer wg.Done(); errs <- request() }(request)
			}
			done := make(chan struct{})
			go func() { wg.Wait(); close(done) }()
			awaitSignal(t, done)
			close(errs)
			for err := range errs {
				if err == nil {
					t.Fatal("competing writer accepted during preparation")
				}
			}
			unblock.Do(func() { close(provider.release) })
			if err := <-first; err != nil {
				t.Fatal(err)
			}
			waitDeployment(t, s)
			s.deployment.wg.Wait()
			if releaseGate, err := s.operations.acquire(); err != nil {
				t.Fatalf("operation reservation leaked: %v", err)
			} else {
				releaseGate()
			}
		})
	}
}

func TestPackageExecutionExcludesDeployment(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var unblock sync.Once
	defer unblock.Do(func() { close(release) })
	runner := &packageRunner{onRun: func() { close(entered); <-release }}
	p, s := newTestServiceState(t, serviceTestCatalog(false), &packageFacts{}, runner, true)
	// The shared gate must reject these before touching a deployment dependency.
	if _, err := s.StartPackageInstall([]string{"testapp"}); err != nil {
		t.Fatal(err)
	}
	awaitSignal(t, entered)
	if _, err := s.StartDeployment("selftest", nil); err == nil {
		t.Fatal("deployment overlapped package execution")
	}
	if _, err := s.ResumeDeployment("session", false); err == nil {
		t.Fatal("resume overlapped package execution")
	}
	unblock.Do(func() { close(release) })
	waitForInstall(t, p)
	p.wg.Wait()
	if _, err := s.StartPackageInstall(nil); err == nil {
		t.Fatal("empty selection accepted")
	}
	if releaseGate, err := s.operations.acquire(); err != nil {
		t.Fatalf("failed request leaked reservation: %v", err)
	} else {
		releaseGate()
	}
}

func TestShutdownDuringDeploymentPreparation(t *testing.T) {
	for _, resume := range []bool{false, true} {
		t.Run(map[bool]string{false: "start", true: "resume"}[resume], func(t *testing.T) {
			s := newDeploymentTestService(t)
			id := pendingSession(t, s)
			provider := &preparationProvider{entered: make(chan struct{}), release: make(chan struct{})}
			s.provider = provider
			var unblock sync.Once
			defer unblock.Do(func() { close(provider.release) })
			result := make(chan error, 1)
			go func() {
				var err error
				if resume {
					_, err = s.ResumeDeployment(id, false)
				} else {
					_, err = s.StartDeployment("selftest", nil)
				}
				result <- err
			}()
			awaitSignal(t, provider.entered)
			if err := s.ServiceShutdown(); err != nil {
				t.Fatal(err)
			}
			unblock.Do(func() { close(provider.release) })
			if err := <-result; err == nil {
				t.Fatal("writer registered after shutdown")
			}
			if s.GetDeployment().Generation != 0 {
				t.Fatal("shutdown started a run")
			}
			if _, err := s.StartPackageInstall([]string{"sevenzip"}); err == nil {
				t.Fatal("package accepted after shutdown")
			}
		})
	}
}
