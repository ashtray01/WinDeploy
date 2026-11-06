import { chromium } from '../desktop/modern/frontend/node_modules/playwright-core/index.mjs';
import fs from 'node:fs/promises';

let browser;
for (let attempt = 0; attempt < 40; attempt++) {
  try { browser = await chromium.connectOverCDP('http://127.0.0.1:9223', { timeout: 2000 }); break; }
  catch { await new Promise(resolve => setTimeout(resolve, 250)); }
}
if (!browser) throw new Error('Desktop WebView2 CDP was not available');
const pages = browser.contexts().flatMap(context => context.pages());
const page = pages.find(page => page.url() !== 'about:blank') ?? pages[0];
if (!page) throw new Error('No desktop WebView page');
await page.getByRole('heading', { name: 'WinDeploy', exact: true }).waitFor();
await fs.mkdir('local', { recursive: true });
await page.screenshot({ path: 'local/modern-bootstrap.png' });
console.log(JSON.stringify({ desktopPage: true, title: await page.title(), heading: 'WinDeploy' }));
await browser.close();
