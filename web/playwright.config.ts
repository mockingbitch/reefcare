import { defineConfig } from '@playwright/test';

// End-to-end tests run the real Go API against the compose PostgreSQL and read emails from
// Mailpit. Prerequisites: `docker compose up -d db mailpit` (see web/README.md).
// Ports are chosen so they don't collide with `make up` (API :4000, web dev :4001).
const apiPort = Number(process.env.E2E_API_PORT ?? 4010);
const webPort = Number(process.env.E2E_WEB_PORT ?? 4011);
const smtpPort = process.env.MAILPIT_SMTP_PORT ?? '4004';
const databaseURL =
	process.env.E2E_DATABASE_URL ??
	'postgres://reefcare:reefcare@localhost:4002/reefcare?sslmode=disable';

export default defineConfig({
	testDir: 'e2e',
	testMatch: '**/*.e2e.ts',
	fullyParallel: false,
	workers: 1,
	retries: process.env.CI ? 1 : 0,
	use: {
		baseURL: `http://localhost:${webPort}`,
		locale: 'vi-VN', // Vietnamese first
		timezoneId: 'Asia/Ho_Chi_Minh',
		trace: 'retain-on-failure'
	},
	webServer: [
		{
			command: 'go run ./cmd/api migrate up && go run ./cmd/api serve',
			cwd: '..',
			port: apiPort,
			reuseExistingServer: false,
			timeout: 180_000,
			env: {
				APP_ENV: 'development',
				HTTP_ADDR: `:${apiPort}`,
				DATABASE_URL: databaseURL,
				PUBLIC_BASE_URL: `http://localhost:${webPort}`,
				SMTP_HOST: 'localhost',
				SMTP_PORT: smtpPort,
				MAIL_FROM: 'ReefCare <no-reply@reefcare.local>',
				LOG_FORMAT: 'text',
				LOG_LEVEL: 'warn'
			}
		},
		{
			command: 'npm run build && npm run preview -- --port ' + webPort + ' --strictPort',
			port: webPort,
			reuseExistingServer: !process.env.CI,
			timeout: 180_000,
			env: { API_URL: `http://localhost:${apiPort}` }
		}
	]
});
