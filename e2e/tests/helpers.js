const { expect } = require("@playwright/test");
const AxeBuilder = require("@axe-core/playwright").default;

/**
 * AC28: no WCAG 2.2 AA violations on the page as it is now.
 *
 * axe-core is JavaScript, and the tests run with JavaScript off (AC29), so the
 * page's HTML (whatever state it is in, including a POST result) is rendered
 * again in a JavaScript-enabled context at the same width and axe runs there.
 * A <base> makes the stylesheet load so colour-contrast is checked for real.
 */
async function expectAccessible(page) {
  const html = (await page.content()).replace(/<head>/i, `<head><base href="${page.url()}">`);
  const context = await page.context().browser().newContext({
    javaScriptEnabled: true,
    viewport: page.viewportSize(),
    ignoreHTTPSErrors: true,
  });
  try {
    const axePage = await context.newPage();
    await axePage.setContent(html, { waitUntil: "load" });
    // The stylesheet must have applied, or colour contrast is checked on
    // browser defaults; and axe must have checked something (rule 9).
    const accent = await axePage.evaluate(() => getComputedStyle(document.documentElement).getPropertyValue("--accent").trim());
    expect(accent, "site stylesheet did not load in the axe page").not.toBe("");
    const results = await new AxeBuilder({ page: axePage })
      .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa", "best-practice"])
      .analyze();
    expect(results.passes.length, "axe checked nothing").toBeGreaterThan(0);
    expect(results.violations, JSON.stringify(results.violations, null, 1)).toEqual([]);
  } finally {
    await context.close();
  }
}

/** AC28: at 320 CSS pixels (400% zoom) nothing scrolls sideways. */
async function expectNoHorizontalScroll(page) {
  const { scrollWidth, clientWidth } = await page.evaluate(() => ({
    scrollWidth: document.documentElement.scrollWidth,
    clientWidth: document.documentElement.clientWidth,
  }));
  expect(scrollWidth, "page scrolls horizontally").toBeLessThanOrEqual(clientWidth);
}

/** Both checks, for every state a test reaches. */
async function expectPageOK(page) {
  await expectAccessible(page);
  await expectNoHorizontalScroll(page);
}

/** The focused element must show a visible focus indicator (AC28). */
async function expectVisibleFocus(page) {
  const style = await page.evaluate(() => {
    const el = document.activeElement;
    const s = getComputedStyle(el);
    return { tag: el.tagName, outlineStyle: s.outlineStyle, outlineWidth: s.outlineWidth };
  });
  expect(style.outlineStyle, `focus on ${style.tag} has no outline`).not.toBe("none");
  expect(style.outlineStyle, `focus on ${style.tag} relies on the browser's default ring`).not.toBe("auto");
  expect(parseFloat(style.outlineWidth)).toBeGreaterThanOrEqual(3);
}

module.exports = { expectAccessible, expectNoHorizontalScroll, expectPageOK, expectVisibleFocus };
