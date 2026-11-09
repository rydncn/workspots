// Contributing (AC13 to AC16, AC22) with JavaScript off, every state under axe.
const { test, expect } = require("@playwright/test");
const { expectPageOK, expectVisibleFocus } = require("./helpers");

const password = "a long enough password";
const contributor = (testInfo) => `contributor-${testInfo.project.name}@example.test`;

async function signIn(page, email) {
  await page.goto("/sign-in");
  await page.getByLabel("Email address").fill(email);
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page).toHaveURL(/\/account$/);
}

test("adding a spot needs an account and comes back to the form (AC13)", async ({ page }, testInfo) => {
  await page.goto("/spots/new");
  await expect(page).toHaveURL(/\/sign-in\?next=%2Fspots%2Fnew/);
  await expectPageOK(page);
  await page.getByLabel("Email address").fill(contributor(testInfo));
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page).toHaveURL(/\/spots\/new$/);
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Add a spot");
  await expectPageOK(page);
});

test("add a spot: validation, then pending and visible only to its author (AC14, AC15)", async ({ page, browser }, testInfo) => {
  await signIn(page, contributor(testInfo));
  await page.goto("/spots/new");
  await page.getByLabel("Address").fill("3 Kept Lane");
  await page.getByLabel("Town or postcode district").fill("York");
  // The browser's own `required` check would stop the post; drop it so the server's validation is what we see.
  await page.getByLabel("Name").evaluate((el) => el.removeAttribute("required"));
  await page.getByRole("button", { name: "Add spot" }).click(); // no name: the server must say so
  await expect(page.getByRole("alert")).toContainText("name");
  await expect(page.getByLabel("Address")).toHaveValue("3 Kept Lane");
  await expect(page.getByLabel("Name")).toHaveAttribute("aria-invalid", "true");
  await expectPageOK(page);

  const name = `New Spot ${testInfo.project.name} ${Date.now().toString(36)}`;
  await page.getByLabel("Name").fill(name);
  await page.getByLabel("Kind of place").selectOption("library");
  await page.getByRole("radio", { name: "Good" }).check();
  await page.getByRole("radio", { name: "Quiet", exact: true }).check();
  await page.getByLabel("Power sockets").check();
  await page.getByRole("button", { name: "Add spot" }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText(name);
  await expect(page.getByRole("status")).toContainText("awaiting approval");
  await expectPageOK(page);
  const url = page.url();

  await page.goto("/account");
  await expect(page.getByRole("link", { name })).toBeVisible();
  await expect(page.getByText("pending")).toBeVisible();
  await expectPageOK(page);

  await page.goto("/search?area=york");
  await expect(page.getByRole("link", { name })).toHaveCount(0);

  const visitor = await browser.newContext({ javaScriptEnabled: false });
  const other = await visitor.newPage();
  const res = await other.goto(url);
  expect(res.status()).toBe(404);
  await visitor.close();
});

test("leave a report and see it at once (AC16)", async ({ page }, testInfo) => {
  await signIn(page, contributor(testInfo));
  await page.goto("/search?area=bradford");
  await page.getByRole("link", { name: "Reportable Café" }).click();
  await page.getByRole("link", { name: "Add a report" }).click();
  await expectPageOK(page);
  await page.getByRole("radio", { name: "Excellent" }).check();
  await page.getByRole("radio", { name: "Quiet", exact: true }).check();
  await page.getByLabel("Quiet space").check();
  const note = `Great desks by the window (${testInfo.project.name})`;
  await page.getByLabel(/A short note/).fill(note);
  await page.getByRole("button", { name: "Add report" }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Reportable Café");
  await expect(page.getByText(note)).toBeVisible();
  await expectPageOK(page);
});

test("report a problem with a spot (AC22)", async ({ page }, testInfo) => {
  await signIn(page, contributor(testInfo));
  await page.goto("/search?area=york");
  await page.getByRole("link", { name: /Extraordinarily Long Named/ }).click();
  await page.getByRole("link", { name: "Report a problem with this spot" }).click();
  await expectPageOK(page);
  await page.getByLabel("What is the problem?").evaluate((el) => el.removeAttribute("required"));
  await page.getByRole("button", { name: "Send to moderators" }).click(); // empty reason
  await expect(page.getByRole("alert")).toContainText("problem");
  await expectPageOK(page);
  await page.getByLabel("What is the problem?").fill("It closed down in the spring.");
  await page.getByRole("button", { name: "Send to moderators" }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Thank you");
  await expectPageOK(page);
});

test("add a report by keyboard only (AC28)", async ({ page }, testInfo) => {
  await signIn(page, contributor(testInfo));
  await page.goto("/search?area=bradford");
  await page.getByRole("link", { name: "Reportable Café" }).click();
  await page.getByRole("link", { name: "Add a report" }).click();
  await expect(page.getByRole("heading", { level: 1 })).toContainText("Reportable Café");
  // Tab reaches a radio group at its checked member; arrows move within it.
  const wifi = page.locator('input[name="wifi"]:focus');
  for (let i = 0; i < 12 && (await wifi.count()) === 0; i++) {
    await page.keyboard.press("Tab");
  }
  await expect(wifi).toHaveCount(1);
  await expectVisibleFocus(page);
  await page.keyboard.press("ArrowRight");
  await expect(page.getByRole("radio", { name: "Good" })).toBeChecked();
  await page.keyboard.press("Tab");
  await expect(page.locator('input[name="noise"]:focus')).toHaveCount(1);
  await page.keyboard.press("ArrowLeft");
  await expect(page.getByRole("radio", { name: "Quiet", exact: true })).toBeChecked();
  const button = page.getByRole("button", { name: "Add report" });
  for (let i = 0; i < 12 && !(await button.evaluate((el) => el === document.activeElement)); i++) {
    await page.keyboard.press("Tab");
  }
  await expect(button).toBeFocused();
  await expectVisibleFocus(page);
  await page.keyboard.press("Enter");
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Reportable Café");
});
