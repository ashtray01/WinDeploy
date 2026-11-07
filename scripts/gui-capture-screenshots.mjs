// Captures public, anonymized screenshots from the actual running WinDeploy EXE.
// The app must be started separately with --smoke-port 9223 and an isolated
// WINDEPLOY_STATE_DIR. The real hostname is replaced only for the screenshot;
// no backend result is fabricated.
import { chromium } from '../desktop/modern/frontend/node_modules/playwright-core/index.mjs';
import { mkdir } from 'node:fs/promises';
for (const key of ['HTTP_PROXY', 'HTTPS_PROXY', 'ALL_PROXY', 'http_proxy', 'https_proxy', 'all_proxy']) delete process.env[key];
let browser;
for (let i = 0; i < 60; i++) {
  try { browser = await chromium.connectOverCDP('http://127.0.0.1:9223', { timeout: 2000 }); break; } catch { await new Promise(resolve => setTimeout(resolve, 250)); }
}
if (!browser) throw new Error('Desktop WebView2 unavailable');
try {
  let page;
  for (let i = 0; i < 60; i++) {
    page = browser.contexts().flatMap(context => context.pages()).find(candidate => candidate.url() !== 'about:blank');
    if (page) break;
    await new Promise(resolve => setTimeout(resolve, 250));
  }
  if (!page) throw new Error('WinDeploy page did not appear');
  page.setDefaultTimeout(20000);
  await page.setViewportSize({ width: 1366, height: 768 });
  await mkdir('docs/images', { recursive: true });
  const anonymize = async () => {
    await page.evaluate(() => {
      const host = document.querySelector('.local-device strong');
      if (host) host.textContent = 'DEMO-WORKSTATION';
    });
  };
  const nav = page.getByRole('navigation', { name: 'Основная навигация' });
  await nav.getByRole('button', { name: 'Установка ПО', exact: true }).click();
  await page.getByRole('heading', { name: 'Каталог программ', exact: true }).waitFor();
  await page.getByRole('row').filter({ hasText: '7-Zip' }).first().waitFor();
  await anonymize();
  await page.screenshot({ path: 'docs/images/catalog-beta.png' });
  await nav.getByRole('button', { name: 'Выполнение', exact: true }).click();
  await page.getByRole('heading', { name: 'Выполнение профиля', exact: true }).waitFor();
  await page.getByLabel('Профиль для выполнения').waitFor();
  await page.getByRole('button', { name: 'Проверить план', exact: true }).click();
  await page.locator('.deploy-preview-result').waitFor();
  await anonymize();
  await page.screenshot({ path: 'docs/images/execution-beta.png' });
  console.log(JSON.stringify({ actualDesktop: true, anonymizedHostname: true, screenshots: ['docs/images/catalog-beta.png', 'docs/images/execution-beta.png'] }));
} finally {
  await browser.close();
}
