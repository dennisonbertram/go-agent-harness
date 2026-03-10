---
name: cypress-testing
description: "Run end-to-end and component tests with Cypress: npx cypress open, npx cypress run, component testing, network intercepting, custom commands. Trigger: when using Cypress, cypress run, cypress open, e2e testing, component testing, cy.intercept, Cypress custom commands, browser testing"
version: 1
argument-hint: "[open|run] [--spec path] [--browser chrome]"
allowed-tools:
  - bash
  - read
  - write
  - grep
  - glob
---
# Cypress Testing

You are now operating in Cypress end-to-end and component testing mode.

## Installation

```bash
# Install Cypress as a dev dependency
npm install --save-dev cypress

# Install with specific version
npm install --save-dev cypress@13

# Open Cypress for the first time (installs binaries, creates config)
npx cypress open

# Verify installation
npx cypress verify

# Check Cypress version
npx cypress version
```

## Running Tests

```bash
# Open Cypress Test Runner (interactive GUI)
npx cypress open

# Run all tests headlessly (CI mode)
npx cypress run

# Run a specific test file
npx cypress run --spec "cypress/e2e/login.cy.js"

# Run multiple spec files with glob pattern
npx cypress run --spec "cypress/e2e/auth/*.cy.js"

# Run tests in a specific browser
npx cypress run --browser chrome
npx cypress run --browser firefox
npx cypress run --browser edge
npx cypress run --browser electron  # default headless

# Run in headed mode (browser visible)
npx cypress run --headed --browser chrome

# Run component tests
npx cypress run --component

# Run e2e tests explicitly
npx cypress run --e2e

# Run with a specific base URL
npx cypress run --config baseUrl=http://localhost:3000

# Record results to Cypress Cloud
npx cypress run --record --key $CYPRESS_RECORD_KEY

# Run in parallel with Cypress Cloud
npx cypress run --record --key $CYPRESS_RECORD_KEY --parallel
```

## Test File Structure

```javascript
// cypress/e2e/login.cy.js
describe('Login', () => {
  beforeEach(() => {
    // Runs before each test
    cy.visit('/login')
  })

  it('logs in with valid credentials', () => {
    cy.get('[data-cy=email]').type('user@example.com')
    cy.get('[data-cy=password]').type('password123')
    cy.get('[data-cy=submit]').click()

    cy.url().should('include', '/dashboard')
    cy.get('[data-cy=welcome-message]').should('contain', 'Welcome')
  })

  it('shows error for invalid credentials', () => {
    cy.get('[data-cy=email]').type('wrong@example.com')
    cy.get('[data-cy=password]').type('wrongpassword')
    cy.get('[data-cy=submit]').click()

    cy.get('[data-cy=error-message]')
      .should('be.visible')
      .and('contain', 'Invalid credentials')
  })
})
```

## Component Testing

```javascript
// cypress/component/Button.cy.jsx
import Button from '../../src/components/Button'

describe('Button Component', () => {
  it('renders with the correct label', () => {
    cy.mount(<Button label="Click me" />)
    cy.get('button').should('contain', 'Click me')
  })

  it('calls onClick when clicked', () => {
    const onClick = cy.stub().as('onClick')
    cy.mount(<Button label="Click me" onClick={onClick} />)
    cy.get('button').click()
    cy.get('@onClick').should('have.been.calledOnce')
  })

  it('is disabled when disabled prop is true', () => {
    cy.mount(<Button label="Submit" disabled />)
    cy.get('button').should('be.disabled')
  })
})
```

## Network Intercepting

```javascript
// Intercept API calls and stub responses
describe('User Profile', () => {
  it('displays user data from API', () => {
    // Intercept and stub the API call
    cy.intercept('GET', '/api/user/profile', {
      statusCode: 200,
      body: {
        name: 'Jane Doe',
        email: 'jane@example.com',
        role: 'admin'
      }
    }).as('getProfile')

    cy.visit('/profile')
    cy.wait('@getProfile')

    cy.get('[data-cy=user-name]').should('contain', 'Jane Doe')
    cy.get('[data-cy=user-role]').should('contain', 'admin')
  })

  it('handles API errors gracefully', () => {
    cy.intercept('GET', '/api/user/profile', {
      statusCode: 500,
      body: { error: 'Internal Server Error' }
    }).as('getProfileError')

    cy.visit('/profile')
    cy.wait('@getProfileError')

    cy.get('[data-cy=error-banner]').should('be.visible')
  })

  it('intercepts POST requests', () => {
    cy.intercept('POST', '/api/user/profile', (req) => {
      // Assert request body
      expect(req.body).to.have.property('name', 'John Updated')
      req.reply({ statusCode: 200, body: { success: true } })
    }).as('updateProfile')

    cy.visit('/profile/edit')
    cy.get('[data-cy=name-input]').clear().type('John Updated')
    cy.get('[data-cy=save-button]').click()
    cy.wait('@updateProfile')
  })

  it('intercepts with route matching', () => {
    // Match by URL pattern
    cy.intercept('GET', '/api/users/*').as('getUser')
    cy.intercept('GET', { url: '/api/**', method: 'GET' }).as('anyGet')

    // Intercept and modify the real response
    cy.intercept('GET', '/api/config', (req) => {
      req.continue((res) => {
        res.body.featureFlag = true  // inject feature flag
      })
    })
  })
})
```

## Custom Commands

```javascript
// cypress/support/commands.js

// Custom login command
Cypress.Commands.add('login', (email, password) => {
  cy.session([email, password], () => {
    cy.visit('/login')
    cy.get('[data-cy=email]').type(email)
    cy.get('[data-cy=password]').type(password)
    cy.get('[data-cy=submit]').click()
    cy.url().should('include', '/dashboard')
  })
})

// Custom API login (faster than UI login)
Cypress.Commands.add('loginByApi', (email, password) => {
  cy.request({
    method: 'POST',
    url: '/api/auth/login',
    body: { email, password }
  }).then((response) => {
    window.localStorage.setItem('token', response.body.token)
  })
})

// Custom command to check accessibility
Cypress.Commands.add('checkA11y', (selector) => {
  cy.injectAxe()
  cy.checkA11y(selector)
})

// Custom drag-and-drop command
Cypress.Commands.add('dragTo', { prevSubject: 'element' }, (subject, targetSelector) => {
  cy.wrap(subject).trigger('mousedown', { button: 0 })
  cy.get(targetSelector).trigger('mousemove').trigger('mouseup', { force: true })
})

// Usage in tests:
// cy.login('user@example.com', 'password123')
// cy.loginByApi('user@example.com', 'password123')
// cy.get('.draggable').dragTo('.dropzone')
```

## Cypress Configuration

```javascript
// cypress.config.js
const { defineConfig } = require('cypress')

module.exports = defineConfig({
  e2e: {
    baseUrl: 'http://localhost:3000',
    viewportWidth: 1280,
    viewportHeight: 720,
    defaultCommandTimeout: 10000,
    requestTimeout: 10000,
    responseTimeout: 30000,
    video: true,
    screenshotOnRunFailure: true,
    experimentalStudio: true,  // record test actions

    setupNodeEvents(on, config) {
      // Register plugins
      require('@cypress/code-coverage/task')(on, config)
      return config
    },

    // Exclude patterns
    excludeSpecPattern: ['**/examples/**', '**/__snapshots__/**'],

    // Environment variables accessible in tests
    env: {
      apiUrl: 'http://localhost:8080/api',
      username: 'testuser',
    }
  },

  component: {
    devServer: {
      framework: 'react',
      bundler: 'vite'
    },
    specPattern: 'src/**/*.cy.{js,jsx,ts,tsx}'
  }
})
```

## Common Assertions

```javascript
// Element assertions
cy.get('.button').should('be.visible')
cy.get('.button').should('not.exist')
cy.get('.button').should('be.disabled')
cy.get('.button').should('have.class', 'active')
cy.get('.button').should('have.attr', 'href', '/home')
cy.get('.input').should('have.value', 'expected text')
cy.get('.list').should('have.length', 5)

// Text assertions
cy.get('h1').should('contain.text', 'Welcome')
cy.get('p').invoke('text').should('match', /\d+ items/)

// URL assertions
cy.url().should('include', '/dashboard')
cy.url().should('eq', 'http://localhost:3000/login')

// Cookie and localStorage
cy.getCookie('session').should('exist')
cy.window().its('localStorage.token').should('exist')

// Multiple assertions chained
cy.get('[data-cy=user-card]')
  .should('be.visible')
  .and('contain', 'John Doe')
  .and('not.have.class', 'loading')
```

## Fixtures and Test Data

```javascript
// cypress/fixtures/user.json
// {
//   "name": "Test User",
//   "email": "test@example.com",
//   "role": "admin"
// }

// Using fixtures in tests
cy.fixture('user').then((user) => {
  cy.get('[data-cy=name]').type(user.name)
  cy.get('[data-cy=email]').type(user.email)
})

// Intercept with fixture
cy.intercept('GET', '/api/user', { fixture: 'user.json' }).as('getUser')
```

## CI/CD Integration

```bash
# Install dependencies and run tests
npm ci
npx cypress run --browser chrome --headless

# With environment variables
CYPRESS_BASE_URL=https://staging.example.com \
  CYPRESS_API_TOKEN=secret \
  npx cypress run

# Run with retries for flaky tests
npx cypress run --config retries=2

# Generate JUnit XML report for CI
npx cypress run --reporter junit --reporter-options "mochaFile=results/test-[hash].xml"
```

```yaml
# GitHub Actions
- name: Run Cypress tests
  uses: cypress-io/github-action@v6
  with:
    build: npm run build
    start: npm start
    browser: chrome
    record: true
  env:
    CYPRESS_RECORD_KEY: ${{ secrets.CYPRESS_RECORD_KEY }}
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

## Troubleshooting

```bash
# Clear Cypress cache
npx cypress cache clear
npx cypress cache prune  # remove old versions

# Verify Cypress binary
npx cypress verify

# Run with debug output
DEBUG=cypress:* npx cypress run

# Common issues:
# Tests timing out — increase defaultCommandTimeout in cypress.config.js
# Flaky tests — use cy.intercept() to control network, add retries
# Element not found — check selector, add cy.wait() or use .should('be.visible')
# CORS errors — configure proxy in cypress.config.js or intercept requests
# Login required — use cy.session() or loginByApi custom command
```
