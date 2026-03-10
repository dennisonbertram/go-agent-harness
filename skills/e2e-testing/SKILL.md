---
name: e2e-testing
description: "Run Playwright end-to-end browser tests for web applications with screenshots and traces. Trigger: when running Playwright tests, E2E browser testing, UI testing, Playwright setup, recording browser tests, Cypress"
version: 1
argument-hint: "[--grep <pattern>] [--project <browser>]"
allowed-tools:
  - bash
  - read
  - write
  - grep
  - glob
---
# E2E Testing (Playwright)

You are now operating in Playwright end-to-end testing mode.

## Installation and Setup

```bash
# Install Playwright and browsers
npm init playwright@latest

# Or add to existing project
npm install -D @playwright/test
npx playwright install

# Install specific browsers only
npx playwright install chromium
npx playwright install chromium firefox
```

## Running Tests

```bash
# Run all tests (headless by default)
npx playwright test

# Run with visible browser (headed mode)
npx playwright test --headed

# Run a specific test file
npx playwright test tests/login.spec.ts

# Run tests matching a pattern
npx playwright test --grep "user can login"
npx playwright test --grep "@smoke"

# Run tests in a specific browser
npx playwright test --project=chromium
npx playwright test --project=firefox
npx playwright test --project=webkit

# Run in all browsers
npx playwright test --project=chromium --project=firefox --project=webkit

# Run in debug mode (pauses on failure)
npx playwright test --debug
```

## Reporters and Output

```bash
# HTML report (generates report/ directory)
npx playwright test --reporter=html

# Open HTML report in browser
npx playwright show-report

# Line reporter (concise output)
npx playwright test --reporter=line

# JSON report
npx playwright test --reporter=json > results.json

# Multiple reporters
npx playwright test --reporter=html,line
```

## Trace and Screenshots

```bash
# Capture traces on failure (default: on-first-retry)
npx playwright test --trace on          # always capture
npx playwright test --trace retain-on-failure  # only on failure
npx playwright test --trace off         # disable

# Open a trace file
npx playwright show-trace trace.zip

# Take screenshots on failure
# Configure in playwright.config.ts:
# screenshot: 'only-on-failure'
```

## Code Generation (Record Tests)

```bash
# Record interactions and generate test code
npx playwright codegen http://localhost:3000

# Record with authentication state saved
npx playwright codegen --save-storage=auth.json http://localhost:3000

# Use saved auth state
npx playwright codegen --load-storage=auth.json http://localhost:3000
```

## Test File Structure

```typescript
// tests/login.spec.ts
import { test, expect } from '@playwright/test';

test.describe('Authentication', () => {
  test('user can log in with valid credentials', async ({ page }) => {
    await page.goto('/login');
    await page.fill('[name=email]', 'test@example.com');
    await page.fill('[name=password]', 'password');
    await page.click('button[type=submit]');
    await expect(page).toHaveURL('/dashboard');
    await expect(page.getByRole('heading')).toContainText('Dashboard');
  });

  test('shows error for invalid credentials', async ({ page }) => {
    await page.goto('/login');
    await page.fill('[name=email]', 'wrong@example.com');
    await page.fill('[name=password]', 'wrongpassword');
    await page.click('button[type=submit]');
    await expect(page.getByRole('alert')).toBeVisible();
    await expect(page.getByRole('alert')).toContainText('Invalid credentials');
  });
});
```

## Page Object Model

```typescript
// pages/LoginPage.ts
import { Page, Locator } from '@playwright/test';

export class LoginPage {
  readonly page: Page;
  readonly emailInput: Locator;
  readonly passwordInput: Locator;
  readonly submitButton: Locator;
  readonly errorMessage: Locator;

  constructor(page: Page) {
    this.page = page;
    this.emailInput = page.locator('[name=email]');
    this.passwordInput = page.locator('[name=password]');
    this.submitButton = page.locator('button[type=submit]');
    this.errorMessage = page.getByRole('alert');
  }

  async login(email: string, password: string) {
    await this.page.goto('/login');
    await this.emailInput.fill(email);
    await this.passwordInput.fill(password);
    await this.submitButton.click();
  }
}

// Usage in test:
// const loginPage = new LoginPage(page);
// await loginPage.login('test@example.com', 'password');
```

## Configuration (playwright.config.ts)

```typescript
import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  timeout: 30_000,
  retries: process.env.CI ? 2 : 0,
  reporter: process.env.CI ? 'github' : 'html',
  use: {
    baseURL: process.env.BASE_URL ?? 'http://localhost:3000',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
    { name: 'firefox', use: { ...devices['Desktop Firefox'] } },
  ],
});
```

## CI Integration

```yaml
# .github/workflows/e2e.yml
- name: Run Playwright tests
  run: npx playwright test
  env:
    BASE_URL: http://localhost:3000

- name: Upload Playwright report
  if: always()
  uses: actions/upload-artifact@v4
  with:
    name: playwright-report
    path: playwright-report/
```

## Common Selectors

```typescript
// By role (preferred — accessible)
page.getByRole('button', { name: 'Submit' })
page.getByRole('heading', { name: 'Dashboard' })
page.getByRole('link', { name: 'Sign in' })

// By label (accessible)
page.getByLabel('Email address')
page.getByLabel('Password')

// By placeholder
page.getByPlaceholder('Enter your email')

// By test ID (explicit, stable)
page.getByTestId('submit-button')  // data-testid="submit-button"

// CSS selector (fallback)
page.locator('[name=email]')
page.locator('.error-message')
```

## Debugging Tips

```bash
# Step through test in debug mode
PWDEBUG=1 npx playwright test tests/login.spec.ts

# Slow down execution for observation
npx playwright test --headed --slowMo=1000

# Run only tests tagged @only
npx playwright test --grep "@only"
```
