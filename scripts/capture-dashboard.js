const { chromium } = require('playwright');

(async () => {
  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage();
  
  page.on('console', msg => console.log('PAGE LOG:', msg.text()));
  page.on('pageerror', err => console.log('PAGE ERROR:', err.message));
  
  await page.goto('http://localhost:5173/');
  
  await page.fill('[data-testid="sign-in-email"]', 'my@example.com');
  await page.fill('[data-testid="sign-in-password"]', 'pwd123456');
  
  await page.click('button[type="submit"]');
  
  await page.waitForTimeout(8000);
  
  const url = page.url();
  console.log('Current URL:', url);
  
  const title = await page.title();
  console.log('Page title:', title);
  
  await page.screenshot({ path: '.sisyphus/evidence/baseline/dashboard-full.png', fullPage: true });
  
  console.log('Full dashboard screenshot captured');
  
  await browser.close();
})();