// Connects to the actual Windows WinDeploy diagnostic/smoke EXE launched with
// --smoke-port 9223. The production EXE does not register that flag.
// No test server, fixture backend, downloaded browser or system mutation.
import { chromium } from '../desktop/modern/frontend/node_modules/playwright-core/index.mjs';
import { execFileSync } from 'node:child_process';
import assert from 'node:assert/strict';
for (const key of ['HTTP_PROXY', 'HTTPS_PROXY', 'ALL_PROXY', 'http_proxy', 'https_proxy', 'all_proxy']) delete process.env[key];
let browser;
for (let i = 0; i < 60; i++) { try { browser = await chromium.connectOverCDP('http://127.0.0.1:9223', { timeout: 2000 }); break; } catch { await new Promise(resolve => setTimeout(resolve, 250)); } }
if (!browser) throw new Error('Desktop WebView2 unavailable');
try {
  let page;
  for (let i = 0; i < 60; i++) {
    page = browser.contexts().flatMap(context => context.pages()).find(page => page.url() !== 'about:blank');
    if (page) break;
    await new Promise(resolve => setTimeout(resolve, 250));
  }
  assert(page);
  page.setDefaultTimeout(15000);
  const errors = [];
  page.on('pageerror', e => errors.push(e.message));
  await page.setViewportSize({ width: 1366, height: 768 });
  const nav = page.getByRole('navigation', { name: 'Основная навигация' });
  await nav.getByRole('button', { name: 'Обзор', exact: true }).click();
  await page.waitForFunction(() => { const e = document.querySelector('[data-testid="app-count"]'); return e && e.textContent !== '—'; });
  const native = JSON.parse(execFileSync('bin/WinDeploy-Inspect-Windows-x64.exe', ['inventory', '--format', 'json'], { encoding: 'utf8', windowsHide: true }));
  assert.equal((await page.getByTestId('machine-name').textContent()).toLowerCase(), native.system.hostname.toLowerCase());
  assert.equal(Number(await page.getByTestId('app-count').textContent()), native.apps.length);
  const old = Number((await page.getByTestId('scan-generation').textContent()).replace('#', ''));
  await page.getByRole('button', { name: 'Обновить данные' }).click();
  await page.waitForFunction(old => Number(document.querySelector('[data-testid="scan-generation"]').textContent.replace('#', '')) > old, old);
  await page.getByRole('button', { name: 'Обновить данные' }).waitFor();
  await page.screenshot({ path: 'local/modern-dashboard-final.png' });
  await nav.getByRole('button', { name: 'Программы', exact: true }).click();
  assert(await page.getByRole('row').count() > 1);
  if (native.apps.length) {
    await page.getByRole('textbox', { name: 'Поиск в таблице' }).fill(native.apps[0].name);
    assert((await page.getByRole('row').allTextContents()).some(text => text.includes(native.apps[0].name)));
  }
  await page.getByRole('button', { name: 'Сортировать: Программа', exact: true }).click();
  assert.equal(await page.getByRole('columnheader').first().getAttribute('aria-sort'), 'descending');
  for (const [name, heading] of [['Сеть', 'Сетевые интерфейсы'], ['Накопители', 'Накопители'], ['Система', 'Системные характеристики']]) {
    await nav.getByRole('button', { name, exact: true }).click();
    await page.getByRole('heading', { name: heading, exact: true }).waitFor();
    assert(await page.getByRole('row').count() > 1);
  }
  await nav.getByRole('button', { name: 'Выполнение', exact: true }).click();
  await page.getByRole('heading', { name: 'Выполнение профиля', exact: true }).waitFor();
  await page.getByLabel('Профиль для выполнения').waitFor();
  await nav.getByRole('button', { name: 'Профили', exact: true }).click();
  await page.getByRole('button', { name: /^Администратор/ }).click();
  await page.getByRole('heading', { name: 'Администратор', exact: true }).waitFor();
  await page.getByRole('button', { name: 'Построить план', exact: true }).click();
  await page.getByRole('img', { name: 'Граф зависимостей плана' }).waitFor();
  assert(await page.getByTestId('dag-edge').count() > 0);
  assert(await page.getByText('Без выполнения', { exact: true }).isVisible());
  assert(await page.locator('.app-shell').evaluate(e => e.scrollWidth <= e.clientWidth));
  await page.screenshot({ path: 'local/modern-profiles.png' });
  await page.getByRole('button', { name: /^Legacy/ }).click();
  await page.getByRole('heading', { name: 'Legacy', exact: true }).waitFor();
  await page.getByRole('button', { name: 'Построить план', exact: true }).click();
  // Current native host is Windows 11; this does not claim Win7 runtime support.
  assert.equal(native.system.windows.major, 10);
  assert(native.system.windows.build >= 22000);
  await page.getByText('Несовместимо', { exact: true }).waitFor();
  await nav.getByRole('button', { name: 'Установка ПО', exact: true }).click();
  await page.getByRole('heading', { name: 'Каталог программ', exact: true }).waitFor();
  await page.getByRole('button', { name: /Установить/ }).first().waitFor();
  assert(await page.getByRole('row').count() >= 12);
  assert(await page.getByText('WinGet', { exact: true }).first().isVisible());
  assert(await page.getByText('SHA-256', { exact: false }).first().isVisible());
  const checkboxes = page.getByRole('checkbox', { name: /^Выбрать / });
  assert(await checkboxes.count() >= 6);
  await checkboxes.first().uncheck();
  await page.getByRole('button', { name: 'Проверить заново', exact: true }).click();
  await page.getByRole('heading', { name: 'Каталог программ', exact: true }).waitFor();
  await page.screenshot({ path: 'local/modern-packages.png' });
  await nav.getByRole('button', { name: 'Диагностика', exact: true }).click();
  await page.getByRole('heading', { name: 'Диагностика сети', exact: true }).waitFor();
  await page.getByRole('button', { name: 'Проверить', exact: true }).click();
  await page.getByText('DNS · localhost', { exact: true }).waitFor();
  assert(await page.getByRole('row').count() >= 2);
  await nav.getByRole('button', { name: 'Сетевые папки', exact: true }).click();
  await page.getByRole('heading', { name: 'Сетевые папки', exact: true }).waitFor();
  await page.getByLabel('UNC-путь').waitFor();
  await page.getByRole('heading', { name: 'Текущие подключения', exact: true }).waitFor();
  await nav.getByRole('button', { name: 'Драйверы', exact: true }).click();
  await page.getByRole('heading', { name: 'Драйверы', exact: true }).waitFor();
  // The default filter legitimately has no rows on a healthy machine.
  await page.getByRole('button', { name: /^Все \(/ }).click();
  await page.getByRole('row').first().waitFor();
  assert.equal(await page.getByRole('alert').count(), 0);
  assert(await page.getByRole('row').count() >= 1);
  await nav.getByRole('button', { name: 'Принтеры', exact: true }).click();
  await page.getByRole('heading', { name: 'Принтеры', exact: true }).waitFor();
  await page.getByLabel('Имя принтера').waitFor();
  await nav.getByRole('button', { name: 'Браузеры', exact: true }).click();
  await page.getByRole('heading', { name: 'Политики браузеров', exact: true }).waitFor();
  await page.getByLabel('Домашняя страница').waitFor();
  await nav.getByRole('button', { name: 'Windows', exact: true }).click();
  await page.getByRole('heading', { name: 'Настройка Windows', exact: true }).waitFor();
  await page.getByLabel('Имя компьютера').waitFor();
  await page.getByRole('button', { name: 'Журнал', exact: true }).click();
  await page.getByRole('button', { name: 'Обновить журнал', exact: true }).click();
  await page.getByRole('textbox', { name: 'Поиск в таблице' }).fill('Построен предварительный план');
  await page.getByText('Построен предварительный план. Системные изменения не выполнялись.', { exact: true }).first().waitFor();
  await page.setViewportSize({ width: 1024, height: 640 });
  assert(await page.locator('.app-shell').evaluate(e => e.scrollWidth <= e.clientWidth));
  assert.deepEqual(errors, []);
  await nav.getByRole('button', { name: 'Обзор', exact: true }).click();
  await page.setViewportSize({ width: 1366, height: 768 });
  console.log(JSON.stringify({ actualDesktop: true, nativeHostnameMatched: true, nativeAppCountMatched: true, apps: native.apps.length, rescan: true, inventoryTables: true, profileDAG: true, legacyIncompatible: true, packageCatalog: true, packageSelection: true, networkDiagnostics: true, sharesPage: true, driversPage: true, printersPage: true, browserPolicies: true, windowsSettings: true, deploymentPage: true, realJournal: true, viewports: ['1366x768', '1024x640'], pageErrors: errors.length }));
} finally { await browser.close(); }
