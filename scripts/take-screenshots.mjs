import { chromium } from 'playwright-core';
import { fileURLToPath } from 'url';
import { mkdir } from 'fs/promises';
import path from 'path';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const screenshotsDir = path.join(__dirname, '..', 'screenshots');
const BASE_URL = process.env.BASE_URL || 'http://localhost:5173';

function setTheme(page, theme) {
    return page.evaluate((t) => {
        document.documentElement.dataset.theme = t;
    }, theme);
}

async function main() {
    await mkdir(screenshotsDir, { recursive: true });

    const browser = await chromium.launch();
    const context = await browser.newContext({
        viewport: { width: 1440, height: 900 },
        deviceScaleFactor: 2,
    });

    const page = await context.newPage();

    // ── 1. Login — light mode ─────────────────────
    console.log('📸 Login (light)...');
    await page.goto(`${BASE_URL}/#/login`, { waitUntil: 'networkidle' });
    await page.waitForTimeout(1500);
    await setTheme(page, 'light');
    await page.waitForTimeout(500);
    await page.screenshot({ path: path.join(screenshotsDir, 'login-light.png') });

    // ── 2. Login — dark mode ──────────────────────
    console.log('📸 Login (dark)...');
    await setTheme(page, 'dark');
    await page.waitForTimeout(500);
    await page.screenshot({ path: path.join(screenshotsDir, 'login-dark.png') });

    // ── 3. Register — light mode ──────────────────
    console.log('📸 Register (light)...');
    await page.goto(`${BASE_URL}/#/register`, { waitUntil: 'networkidle' });
    await page.waitForTimeout(1500);
    await setTheme(page, 'light');
    await page.waitForTimeout(500);
    await page.screenshot({ path: path.join(screenshotsDir, 'register-light.png') });

    // ── 4. Register — dark mode ───────────────────
    console.log('📸 Register (dark)...');
    await setTheme(page, 'dark');
    await page.waitForTimeout(500);
    await page.screenshot({ path: path.join(screenshotsDir, 'register-dark.png') });

    // ── 5. Mobile — login light ───────────────────
    console.log('📸 Login mobile (light)...');
    await page.setViewportSize({ width: 375, height: 812 });
    await page.goto(`${BASE_URL}/#/login`, { waitUntil: 'networkidle' });
    await page.waitForTimeout(1500);
    await setTheme(page, 'light');
    await page.waitForTimeout(500);
    await page.screenshot({ path: path.join(screenshotsDir, 'login-mobile-light.png') });

    // ── 6. Mobile — login dark ────────────────────
    console.log('📸 Login mobile (dark)...');
    await setTheme(page, 'dark');
    await page.waitForTimeout(500);
    await page.screenshot({ path: path.join(screenshotsDir, 'login-mobile-dark.png') });

    await browser.close();
    console.log('✅ All screenshots saved to screenshots/');
}

main().catch((err) => {
    console.error('Screenshot script failed:', err);
    process.exit(1);
});