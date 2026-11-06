// Connects to the actual Windows EXE launched with --smoke-port 9223.
// No test server, fixture backend, downloaded browser or system mutation.
import { chromium } from '../desktop/modern/frontend/node_modules/playwright-core/index.mjs';
import { execFileSync } from 'node:child_process';
import assert from 'node:assert/strict';
for (const key of ['HTTP_PROXY', 'HTTPS_PROXY', 'ALL_PROXY', 'http_proxy', 'https_proxy', 'all_proxy']) delete process.env[key];
let browser;
for (let i = 0; i < 20; i++) { try { browser = await chromium.connectOverCDP('http://127.0.0.1:9223', { timeout: 2000 }); break; } catch { await new Promise(resolve => setTimeout(resolve, 200)); } }
if (!browser) throw new Error('Desktop WebView2 unavailable');
try {
  const page = browser.contexts().flatMap(context => context.pages()).find(page => page.url() !== 'about:blank');
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
  await nav.getByRole('button', { name: 'Журнал', exact: true }).click();
  await page.getByRole('button', { name: 'Обновить журнал', exact: true }).click();
  await page.getByRole('textbox', { name: 'Поиск в таблице' }).fill('Построен предварительный план');
  await page.getByText('Построен предварительный план. Системные изменения не выполнялись.', { exact: true }).first().waitFor();
  assert.equal(await page.getByRole('button', { name: /Установить|Применить|Выполнить/ }).count(), 0);
  assert.equal(await nav.locator('button.future:disabled').count(), 5);
  await page.setViewportSize({ width: 1024, height: 640 });
  assert(await page.locator('.app-shell').evaluate(e => e.scrollWidth <= e.clientWidth));
  assert.deepEqual(errors, []);
  await nav.getByRole('button', { name: 'Обзор', exact: true }).click();
  await page.setViewportSize({ width: 1366, height: 768 });
  console.log(JSON.stringify({ actualDesktop: true, nativeHostnameMatched: true, nativeAppCountMatched: true, apps: native.apps.length, rescan: true, inventoryTables: true, profileDAG: true, legacyIncompatible: true, realJournal: true, noExecutionControls: true, viewports: ['1366x768', '1024x640'], pageErrors: errors.length }));
} finally { await browser.close(); }
