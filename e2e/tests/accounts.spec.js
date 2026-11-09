// Accounts (AC7 to AC10, AC17) as a person would, JavaScript off (AC29),
// every state checked for accessibility (AC28).
const { test, expect } = require("@playwright/test");
const { expectPageOK, expectVisibleFocus } = require("./helpers");

const password = "a long enough password";
const fresh = () => `e2e-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}@example.test`;
// Seeded accounts (seed.sql), one per project so tests don't share state. The
// in-memory sign-up limit is ten per IP, so only one journey signs up.
const member = (testInfo) => `member-${testInfo.project.name}@example.test`;
const reader = (testInfo) => `reader-${testInfo.project.name}@example.test`;

async function signUp(page, email) {
  await page.goto("/sign-up");
  await page.getByLabel("Email address").fill(email);
  await page.getByLabel(/^Password/).fill(password);
  await page.getByRole("button", { name: "Create account" }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Check your email, then sign in");
}

async function signIn(page, email, pw = password) {
  await page.goto("/sign-in");
  await page.getByLabel("Email address").fill(email);
  await page.getByLabel("Password").fill(pw);
  await page.getByRole("button", { name: "Sign in" }).click();
}

test("sign up, sign in, see the account, sign out (AC7, AC8, AC9) @smoke", async ({ page }) => {
  const email = fresh();
  await page.goto("/sign-up");
  await expectPageOK(page);
  await signUp(page, email);
  await expectPageOK(page);

  await signIn(page, email);
  await expect(page).toHaveURL(/\/account$/);
  await expect(page.getByText(email)).toBeVisible();
  await expect(page.getByRole("link", { name: "Account" })).toBeVisible();
  await expectPageOK(page);

  await page.getByRole("button", { name: "Sign out" }).first().click();
  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByRole("link", { name: "Sign in" })).toBeVisible();
  const res = await page.goto("/account");
  await expect(page).toHaveURL(/\/sign-in\?next=%2Faccount/);
  expect(res.status()).toBe(200);
});

test("validation failures keep what was typed and are announced (AC7, AC28)", async ({ page }) => {
  await page.goto("/sign-up");
  await page.getByLabel("Email address").fill("kept@example.test");
  await page.getByLabel(/^Password/).fill("short");
  // The browser's own minlength check would stop the post; submit via the server's view of the form instead.
  await page.getByLabel(/^Password/).evaluate((el) => el.removeAttribute("minlength"));
  await page.getByRole("button", { name: "Create account" }).click();
  await expect(page.getByRole("alert")).toContainText("12 characters");
  await expect(page.getByLabel("Email address")).toHaveValue("kept@example.test");
  await expect(page.getByLabel(/^Password/)).toHaveAttribute("aria-invalid", "true");
  await expectPageOK(page);
});

test("a wrong password and an unknown address are refused alike (AC8)", async ({ page }, testInfo) => {
  await signIn(page, reader(testInfo), "not the password at all");
  const first = await page.getByRole("alert").textContent();
  await signIn(page, fresh(), password);
  const second = await page.getByRole("alert").textContent();
  expect(first).toBe(second);
  await expect(page).toHaveURL(/\/sign-in$/);
  await expectPageOK(page);
});

test("change password, old one refused, new one works (AC10)", async ({ page }, testInfo) => {
  const email = member(testInfo); // this account's password changes; nothing else signs in as it
  await signIn(page, email);
  await page.getByRole("link", { name: "Change password" }).click();
  await expectPageOK(page);
  await page.getByLabel("Current password").fill("wrong current one");
  await page.getByLabel(/^New password/).fill("another long password");
  await page.getByRole("button", { name: "Change password" }).click();
  await expect(page.getByRole("alert")).toContainText("current password");
  await expectPageOK(page);
  await page.getByLabel("Current password").fill(password);
  await page.getByLabel(/^New password/).fill("another long password");
  await page.getByRole("button", { name: "Change password" }).click();
  await expect(page.getByRole("status")).toContainText("Your password has been changed");
  await expectPageOK(page);
  await page.getByRole("button", { name: "Sign out" }).first().click();
  await signIn(page, email, password);
  await expect(page.getByRole("alert")).toBeVisible();
  await signIn(page, email, "another long password");
  await expect(page).toHaveURL(/\/account$/);
});

test("signed-in journey by keyboard with visible focus (AC28)", async ({ page }, testInfo) => {
  const email = reader(testInfo);
  await page.goto("/sign-in");
  const nav = page.getByRole("navigation", { name: "Main" });
  await page.keyboard.press("Tab");
  await expect(page.getByRole("link", { name: "Skip to content" })).toBeFocused();
  await expectVisibleFocus(page);
  for (const label of ["Workspots", "Sign in", "Sign up"]) {
    await page.keyboard.press("Tab");
    await expect(nav.getByRole("link", { name: label })).toBeFocused();
    await expectVisibleFocus(page);
  }
  await page.keyboard.press("Tab");
  await expect(page.getByLabel("Email address")).toBeFocused();
  await page.keyboard.type(email);
  await page.keyboard.press("Tab");
  await expect(page.getByLabel("Password")).toBeFocused();
  await expectVisibleFocus(page);
  await page.keyboard.type(password);
  await page.keyboard.press("Enter");
  await expect(page).toHaveURL(/\/account$/);
  // Tab to the Sign out button in the nav and use it.
  const signOut = page.getByRole("button", { name: "Sign out" }).first();
  for (let i = 0; i < 10 && !(await signOut.evaluate((el) => el === document.activeElement)); i++) {
    await page.keyboard.press("Tab");
  }
  await expect(signOut).toBeFocused();
  await expectVisibleFocus(page);
  await page.keyboard.press("Enter");
  await expect(page.getByRole("link", { name: "Sign in" })).toBeVisible();
});
