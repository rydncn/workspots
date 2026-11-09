// Moderation and roles (AC18 to AC21) with JavaScript off, every state under axe.
const { test, expect } = require("@playwright/test");
const { expectPageOK, expectVisibleFocus } = require("./helpers");

const password = "a long enough password";
const who = (role, testInfo) => `${role}-${testInfo.project.name}@example.test`;

async function signIn(page, email) {
  await page.goto("/sign-in");
  await page.getByLabel("Email address").fill(email);
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page).toHaveURL(/\/account$/);
}

test("moderation is for moderators (AC18)", async ({ page }, testInfo) => {
  let res = await page.goto("/moderation");
  expect(res.status()).toBe(401);
  await expectPageOK(page);
  await signIn(page, who("contributor", testInfo)); // a member whose password no journey changes
  res = await page.goto("/moderation");
  expect(res.status()).toBe(403);
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Not allowed");
  await expectPageOK(page);
});

test("approve a spot, by keyboard, and reject another (AC19, AC28)", async ({ page }, testInfo) => {
  // A moderator is also a member: they add the spots, so each project has its own.
  await signIn(page, who("moderator", testInfo));
  const stamp = `${testInfo.project.name} ${Date.now().toString(36)}`;
  for (const verb of ["Approve", "Reject"]) {
    await page.goto("/spots/new");
    await page.getByLabel("Name").fill(`${verb} Me ${stamp}`);
    await page.getByLabel("Town or postcode district").fill("Halifax");
    await page.getByLabel("Address").fill("1 Mill Lane");
    await page.getByRole("button", { name: "Add spot" }).click();
    await expect(page.getByRole("status")).toContainText("awaiting approval");
  }
  await page.getByRole("link", { name: "Moderation" }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Moderation");
  await expectPageOK(page);

  const approve = page.getByRole("button", { name: `Approve Approve Me ${stamp}` });
  for (let i = 0; i < 60 && !(await approve.evaluate((el) => el === document.activeElement)); i++) {
    await page.keyboard.press("Tab");
  }
  await expect(approve).toBeFocused();
  await expectVisibleFocus(page);
  await page.keyboard.press("Enter");
  await expect(page).toHaveURL(/\/moderation$/);
  await expect(approve).toHaveCount(0);
  await page.getByRole("button", { name: `Reject Reject Me ${stamp}` }).click();
  await expect(page.getByRole("button", { name: `Reject Reject Me ${stamp}` })).toHaveCount(0);
  await expectPageOK(page);

  await expect(page.getByRole("listitem").filter({ hasText: `Approve Me ${stamp}` })).toContainText(/approved on .* by moderator-/);
  await page.goto("/search?area=halifax");
  await expect(page.getByRole("link", { name: `Approve Me ${stamp}` })).toBeVisible();
  await expect(page.getByRole("link", { name: `Reject Me ${stamp}` })).toHaveCount(0);
  await page.goto("/account");
  await expect(page.getByText(`Reject Me ${stamp}`).locator("..").locator("..")).toContainText("rejected");
});

test("hide a flagged report with a reason (AC20)", async ({ page }, testInfo) => {
  await signIn(page, who("moderator", testInfo));
  await page.goto("/moderation");
  const note = `FLAGGED NOTE ${testInfo.project.name}`;
  const item = page.getByRole("listitem").filter({ hasText: `Advertising, not a report (${testInfo.project.name}).` });
  await expect(item).toContainText(note);
  await expect(item).toHaveCount(1);
  await item.getByLabel("Why hide this report?").evaluate((el) => el.removeAttribute("required"));
  await item.getByRole("button", { name: "Hide report" }).click(); // no reason
  await expect(page.getByRole("alert")).toContainText("why");
  await expectPageOK(page);
  await item.getByLabel("Why hide this report?").fill(`Advertising (${testInfo.project.name}).`);
  await item.getByRole("button", { name: "Hide report" }).click();
  await expect(page).toHaveURL(/\/moderation$/);
  await expect(item.getByRole("status")).toContainText(`Hidden: Advertising (${testInfo.project.name}).`);
  await expectPageOK(page);
  await item.getByRole("link", { name: "Moderated Café" }).click();
  await expect(page.getByText(note)).toHaveCount(0);
  await page.goto("/moderation");
  await item.getByRole("button", { name: "Mark as resolved" }).click();
  await expect(item).toHaveCount(0);
});

test("only an admin changes roles (AC21)", async ({ page }, testInfo) => {
  await signIn(page, who("moderator", testInfo));
  let res = await page.goto("/admin/users");
  expect(res.status()).toBe(403);
  await page.getByRole("button", { name: "Sign out" }).first().click();

  await signIn(page, who("admin", testInfo));
  await page.getByRole("link", { name: "Roles" }).click();
  await expectPageOK(page);
  await page.getByLabel("Email address of the account").fill(who("promoted", testInfo));
  await page.getByLabel("Role").selectOption("moderator");
  await page.getByRole("button", { name: "Set role" }).click();
  await expect(page.getByRole("status")).toContainText("now has the role moderator");
  await expectPageOK(page);
  await page.getByRole("button", { name: "Sign out" }).first().click();

  await signIn(page, who("promoted", testInfo));
  res = await page.goto("/moderation");
  expect(res.status()).toBe(200);
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Moderation");
});
