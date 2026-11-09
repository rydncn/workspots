// Browse and search (AC2 to AC6) as a person would, with JavaScript off
// (AC29), checking accessibility in every state reached (AC28).
const { test, expect } = require("@playwright/test");
const { expectPageOK, expectVisibleFocus } = require("./helpers");

test("home page lists approved spots and is accessible @smoke", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Find a place to work");
  await expect(page.getByRole("link", { name: "Riverside Café" })).toBeVisible();
  await expect(page.getByText("Pending Place")).toHaveCount(0);
  await expectPageOK(page);
});

test("search by area with filters, then open a spot (AC2, AC3, AC5)", async ({ page }) => {
  await page.goto("/");
  await page.getByLabel("Town or postcode district").fill("leeds");
  await page.getByLabel("Wi-Fi at least").selectOption({ label: "Good" });
  await page.getByLabel("Power sockets").check();
  await page.getByRole("button", { name: "Search" }).click();

  await expect(page).toHaveURL(/\/search\?/);
  await expect(page.getByRole("heading", { level: 2, name: /2 matching spots/ })).toBeVisible();
  await expect(page.getByRole("link", { name: "Riverside Café" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Headingley Library" })).toBeVisible();
  await expect(page.getByLabel("Town or postcode district")).toHaveValue("leeds");
  await expect(page.getByLabel("Power sockets")).toBeChecked();
  await expectPageOK(page);

  await page.getByRole("link", { name: "Riverside Café" }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Riverside Café");
  await expect(page.getByText("From 2 reports")).toBeVisible();
  await expect(page.getByText("Busy at lunch.")).toBeVisible();
  await expect(page.getByText("HIDDEN SPAM NOTE")).toHaveCount(0);
  const reports = page.locator(".reports li");
  await expect(reports).toHaveCount(2);
  await expect(reports.first()).toContainText("Busy at lunch.");
  await expectPageOK(page);
});

test("no match is a page that keeps the search (AC4)", async ({ page }) => {
  await page.goto("/search?area=Nowhere&quiet_space=1");
  await expect(page.getByText("No spots match")).toBeVisible();
  await expect(page.getByLabel("Town or postcode district")).toHaveValue("Nowhere");
  await expect(page.getByLabel("Quiet space")).toBeChecked();
  await expectPageOK(page);
});

test("worst-case data: a very long name and no reports (AC5, AC28)", async ({ page }) => {
  await page.goto("/search?area=york");
  await page.getByRole("link", { name: /Extraordinarily Long Named/ }).click();
  await expect(page.getByText("No reports yet.", { exact: true })).toBeVisible();
  await expect(page.getByText("Not known yet.")).toBeVisible();
  await expectPageOK(page);
});

test("unknown and pending spots are a 404 page with a correlation ID (AC6)", async ({ page }) => {
  const res = await page.goto("/spots/999999");
  expect(res.status()).toBe(404);
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Spot not found");
  await expect(page.getByText(/Correlation ID/)).toBeVisible();
  await expectPageOK(page);
});

test("search journey by keyboard only, with visible focus at every stop (AC28)", async ({ page }) => {
  await page.goto("/");
  const stops = [
    page.getByRole("link", { name: "Skip to content" }),
    page.getByRole("link", { name: "Workspots" }),
    page.getByLabel("Town or postcode district"),
  ];
  for (const stop of stops) {
    await page.keyboard.press("Tab");
    await expect(stop).toBeFocused();
    await expectVisibleFocus(page);
  }
  await page.keyboard.type("LS6");
  // On through the selects and checkboxes to the button, each with a visible ring.
  for (const label of ["Wi-Fi at least", "Noise at most", "Power sockets", "No seating time limit", "Step-free entrance", "Accessible toilet", "Quiet space"]) {
    await page.keyboard.press("Tab");
    await expect(page.getByLabel(label)).toBeFocused();
    await expectVisibleFocus(page);
  }
  await page.keyboard.press("Space"); // tick "Quiet space"
  await page.keyboard.press("Tab");
  await expect(page.getByRole("button", { name: "Search" })).toBeFocused();
  await expectVisibleFocus(page);
  await page.keyboard.press("Enter");

  await expect(page).toHaveURL(/area=LS6/);
  await expect(page.getByLabel("Quiet space")).toBeChecked();
  const result = page.getByRole("link", { name: "Headingley Library" });
  await expect(result).toBeVisible();
  await expect(page.getByRole("link", { name: "Riverside Café" })).toHaveCount(0);
  // Tab until the result link has focus, then open it with Enter.
  for (let i = 0; i < 20 && !(await result.evaluate((el) => el === document.activeElement)); i++) {
    await page.keyboard.press("Tab");
  }
  await expect(result).toBeFocused();
  await expectVisibleFocus(page);
  await page.keyboard.press("Enter");
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Headingley Library");
});
