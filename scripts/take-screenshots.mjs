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

    // ── 5. Register a new user ────────────────────
    console.log('📝 Registering new user...');
    await page.goto(`${BASE_URL}/#/register`, { waitUntil: 'networkidle' });
    await page.waitForTimeout(1500);
    await page.fill('input[autocomplete="name"]', 'Demo User');
    await page.fill('input[autocomplete="email"]', 'demo@veverka.net');
    await page.fill('input[autocomplete="new-password"]', 'password123');
    // confirmPassword is the second new-password field
    const passwordFields = await page.locator('input[autocomplete="new-password"]').all();
    await passwordFields[1].fill('password123');
    await page.click('button[type="submit"]');
    await page.waitForURL(`${BASE_URL}/#/`, { timeout: 10000 });
    await page.waitForTimeout(1500);

    // ── 6. Dashboard — light mode ───────────────────
    console.log('📸 Dashboard (light)...');
    await setTheme(page, 'light');
    await page.waitForTimeout(500);
    await page.screenshot({ path: path.join(screenshotsDir, 'dashboard-light.png') });

    // ── 7. Dashboard — dark mode ────────────────────
    console.log('📸 Dashboard (dark)...');
    await setTheme(page, 'dark');
    await page.waitForTimeout(500);
    await page.screenshot({ path: path.join(screenshotsDir, 'dashboard-dark.png') });

    // ── 8. Shares — light mode ────────────────────
    console.log('📸 Shares (light)...');
    await page.goto(`${BASE_URL}/#/shares`, { waitUntil: 'networkidle' });
    await page.waitForTimeout(1500);
    await setTheme(page, 'light');
    await page.waitForTimeout(500);
    await page.screenshot({ path: path.join(screenshotsDir, 'shares-light.png') });

    // ── 9. Shares — dark mode ─────────────────────
    console.log('📸 Shares (dark)...');
    await setTheme(page, 'dark');
    await page.waitForTimeout(500);
    await page.screenshot({ path: path.join(screenshotsDir, 'shares-dark.png') });

    // ── 10. New Share — light mode ────────────────
    console.log('📸 New Share (light)...');
    await page.goto(`${BASE_URL}/#/shares/new`, { waitUntil: 'networkidle' });
    await page.waitForTimeout(1500);
    await setTheme(page, 'light');
    await page.waitForTimeout(500);
    await page.screenshot({ path: path.join(screenshotsDir, 'new-share-light.png') });

    // ── 11. New Share — dark mode ─────────────────
    console.log('📸 New Share (dark)...');
    await setTheme(page, 'dark');
    await page.waitForTimeout(500);
    await page.screenshot({ path: path.join(screenshotsDir, 'new-share-dark.png') });

    // ── 12. Fill and submit a new share ─────────────
    console.log('📝 Creating a share...');
    await setTheme(page, 'light');
    await page.fill('input[placeholder="My Share"]', 'Test Share');
    await page.fill('#new-share-description', 'A sample share created for demonstration purposes.');
    await page.fill('input[placeholder="my-custom-slug (optional)"]', 'test-share-slug');
    // Attach one of the existing screenshots as a file
    const loginScreenshot = path.join(screenshotsDir, 'login-light.png');
    await page.locator('input[type="file"]').setInputFiles(loginScreenshot);
    await page.waitForTimeout(500);
    await page.screenshot({ path: path.join(screenshotsDir, 'new-share-filled-light.png') });
    await setTheme(page, 'dark');
    await page.waitForTimeout(500);
    await page.screenshot({ path: path.join(screenshotsDir, 'new-share-filled-dark.png') });

    // ── 13. Submit the share ────────────────────────
    console.log('📸 Submitting share...');
    await page.click('button[type="submit"]');
    await page.waitForURL(/\/#\/shares\//, { timeout: 10000 });
    await page.waitForTimeout(1500);

    // ── 14. Share detail — light mode ───────────────
    console.log('📸 Share detail (light)...');
    await setTheme(page, 'light');
    await page.waitForTimeout(500);
    await page.screenshot({ path: path.join(screenshotsDir, 'share-detail-light.png') });

    // ── 15. Share detail — dark mode ────────────────
    console.log('📸 Share detail (dark)...');
    await setTheme(page, 'dark');
    await page.waitForTimeout(500);
    await page.screenshot({ path: path.join(screenshotsDir, 'share-detail-dark.png') });

    // ── 16. Dashboard with shares — light mode ──────
    console.log('📸 Dashboard with shares (light)...');
    await page.goto(`${BASE_URL}/#/`, { waitUntil: 'networkidle' });
    await page.waitForTimeout(1500);
    await setTheme(page, 'light');
    await page.waitForTimeout(500);
    await page.screenshot({ path: path.join(screenshotsDir, 'dashboard-with-shares-light.png') });

    // ── 17. Dashboard with shares — dark mode ───────
    console.log('📸 Dashboard with shares (dark)...');
    await setTheme(page, 'dark');
    await page.waitForTimeout(500);
    await page.screenshot({ path: path.join(screenshotsDir, 'dashboard-with-shares-dark.png') });

    // ── 18. Settings — light mode ───────────────────
    console.log('📸 Settings (light)...');
    await page.goto(`${BASE_URL}/#/settings`, { waitUntil: 'networkidle' });
    await page.waitForTimeout(1500);
    await setTheme(page, 'light');
    await page.waitForTimeout(500);
    await page.screenshot({ path: path.join(screenshotsDir, 'settings-light.png') });

    // ── 19. Settings — switch to dark theme ─────────
    console.log('📸 Settings (dark theme selected)...');
    await page.click('button:has-text("Dark")');
    await page.waitForTimeout(500);
    await page.screenshot({ path: path.join(screenshotsDir, 'settings-dark.png') });

    // ── 20. Settings — 2FA setup QR modal ───────────
    console.log('📸 2FA setup (QR code)...');
    await page.click('button:has-text("Enable Two-Factor Authentication")');
    await page.waitForTimeout(1500);
    await page.screenshot({ path: path.join(screenshotsDir, 'settings-2fa-qr.png') });

    // ── 21. Settings — 2FA verify modal ─────────────
    console.log('📸 2FA setup (verify)...');
    await page.click('button:has-text("Next")');
    await page.waitForTimeout(500);
    await page.screenshot({ path: path.join(screenshotsDir, 'settings-2fa-verify.png') });

    // ── Admin panel screenshots ─────────────────────
    const adminPages = [
        { route: 'admin/users', name: 'admin-users' },
        { route: 'admin/storage', name: 'admin-storage' },
        { route: 'admin/email', name: 'admin-email' },
        { route: 'admin/webhooks', name: 'admin-webhooks' },
        { route: 'admin/files', name: 'admin-files' },
        { route: 'admin/api-keys', name: 'admin-api-keys' },
    ];

    for (const { route, name } of adminPages) {
        console.log(`📸 ${name} (light)...`);
        await page.goto(`${BASE_URL}/#/${route}`, { waitUntil: 'networkidle' });
        await page.waitForTimeout(1500);
        await setTheme(page, 'light');
        await page.waitForTimeout(500);
        await page.screenshot({ path: path.join(screenshotsDir, `${name}-light.png`) });

        console.log(`📸 ${name} (dark)...`);
        await setTheme(page, 'dark');
        await page.waitForTimeout(500);
        await page.screenshot({ path: path.join(screenshotsDir, `${name}-dark.png`) });
    }

    // ── 22. Mobile — login light ───────────────────
    console.log('📸 Login mobile (light)...');
    await page.setViewportSize({ width: 375, height: 812 });
    await page.goto(`${BASE_URL}/#/login`, { waitUntil: 'networkidle' });
    await page.waitForTimeout(1500);
    await setTheme(page, 'light');
    await page.waitForTimeout(500);
    await page.screenshot({ path: path.join(screenshotsDir, 'login-mobile-light.png') });

    // ── 23. Mobile — login dark ────────────────────
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