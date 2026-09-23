import { expect, test } from "@playwright/test";

const destinations = [
  "/courses", "/listening", "/reading", "/mock-exam", "/speaking",
  "/writing", "/library", "/generate", "/profile",
];

test.beforeEach(async ({ page }) => {
  await page.route("**/telemetry/**", route => route.fulfill({ status: 204 }));
  // All API requests stay local to the test, including accidental mutations.
  await page.route("**/graphql", async route => {
    const { query } = route.request().postDataJSON();
    await route.fulfill({ json: { data: { courses: [], sharedTheater: null } } });
    expect(query).not.toMatch(/\bmutation\b/);
  });
});

async function openCourses(page) {
  await page.goto("/courses");
  await expect(page.getByRole("heading", { name: "课程中心", exact: true })).toBeVisible();
  return page.getByRole("navigation", { name: "主导航", exact: true });
}

async function expectPersistent(nav) {
  await expect(nav).toBeVisible();
  await expect(nav).toHaveCSS("opacity", "1");
  await expect(nav).toHaveCSS("pointer-events", "auto");
  await expect(nav.getByRole("link")).toHaveCount(destinations.length);
}

async function expectBottomClearance(page, nav) {
  await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
  const navBox = await nav.boundingBox();
  const cardBox = await page.locator("main > .card").boundingBox();
  expect(cardBox.y + cardBox.height).toBeLessThanOrEqual(navBox.y);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
}

function expectStableNavigationBox(actualBox, initialBox) {
  expect(actualBox).not.toBeNull();
  expect(actualBox.x).toBe(initialBox.x);
  expect(actualBox.width).toBe(initialBox.width);
  expect(Math.abs(actualBox.y - initialBox.y)).toBeLessThanOrEqual(8);
  expect(Math.abs(actualBox.height - initialBox.height)).toBeLessThanOrEqual(8);
}

test("navigation stays visible before and after edge, hover, focus and idle", async ({ page }, testInfo) => {
  const nav = await openCourses(page);
  await expectPersistent(nav);
  expect(await page.evaluate(() => matchMedia("(pointer: coarse)").matches))
    .toBe(testInfo.project.name !== "desktop");
  const initialBox = await nav.boundingBox();
  const { width, height } = page.viewportSize();
  await page.mouse.move(width / 2, height - 1);
  await expectPersistent(nav);
  expectStableNavigationBox(await nav.boundingBox(), initialBox);
  await nav.getByRole("link").first().hover();
  await page.mouse.move(0, 0);
  await nav.getByRole("link").first().focus();
  await page.keyboard.press("Tab");
  await expect(nav.locator('a[href="/listening"]')).toBeFocused();
  await page.getByRole("heading", { name: "课程中心", exact: true }).click();
  // Exceed both the old 600ms hover timeout and 1300ms edge timeout.
  await page.waitForTimeout(1600);
  await expectPersistent(nav);
  expectStableNavigationBox(await nav.boundingBox(), initialBox);
  await expectBottomClearance(page, nav);
  await page.screenshot({ path: testInfo.outputPath("persistent-navigation.png") });
});

test("all nine destinations remain available and route clicks update active state", async ({ page }, testInfo) => {
  const nav = await openCourses(page);
  expect(await nav.getByRole("link").evaluateAll(links => links.map(link => link.getAttribute("href"))))
    .toEqual(destinations);
  const readingLink = nav.locator('a[href="/reading"]');
  if (testInfo.project.use.hasTouch) await readingLink.tap();
  else await readingLink.click();
  await expect(page).toHaveURL(/\/reading$/);
  await expect(page.getByRole("heading", { name: "阅读训练中心", exact: true })).toBeVisible();
  await expect(readingLink).toHaveAttribute("aria-current", "page");
  await expect(readingLink).toHaveClass(/active/);
  await expectPersistent(nav);
  await expectBottomClearance(page, nav);
  await page.screenshot({ path: testInfo.outputPath("reading-navigation.png") });
  await nav.locator('a[href="/courses"]').click();
  await expect(page).toHaveURL(/\/courses$/);
  await expect(nav.locator('a[href="/courses"]')).toHaveAttribute("aria-current", "page");
});

test("login, updates and shared theater still hide navigation", async ({ page }) => {
  for (const path of ["/login", "/updates", "/theater/shared/navigation-fixture"]) {
    await page.goto(path);
    await expect(page.locator("main")).toBeVisible();
    await expect(page).toHaveURL(new RegExp(`${path}$`));
    await expect(page.getByRole("navigation", { name: "主导航", exact: true })).toHaveCount(0);
    const { width, height } = page.viewportSize();
    await page.mouse.move(width / 2, height - 1);
    await expect(page.locator(".mobile-bottom-nav")).toHaveCount(0);
    await expect(page.locator("#root")).not.toHaveCSS("padding-bottom", "160px");
  }
  await expectPersistent(await openCourses(page));
});

test("resize across 768 and 769 keeps navigation and content accessible", async ({ page }, testInfo) => {
  const nav = await openCourses(page);
  for (const width of [768, 769, 390, 1440]) {
    await page.setViewportSize({ width, height: 900 });
    await expectPersistent(nav);
    await expectBottomClearance(page, nav);
    await page.screenshot({ path: testInfo.outputPath(`resize-${width}.png`) });
  }
});
