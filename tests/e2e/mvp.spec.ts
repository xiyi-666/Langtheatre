import { test, expect } from "@playwright/test";

test("login page renders", async ({ page }) => {
  await page.setContent(`
    <main>
      <h1>LinguaQuest</h1>
      <input placeholder="邮箱" />
      <input placeholder="密码" />
      <button>登录</button>
    </main>
  `);
  await expect(page.getByText("LinguaQuest")).toBeVisible();
  await expect(page.getByPlaceholder("邮箱")).toBeVisible();
  await expect(page.getByPlaceholder("密码")).toBeVisible();
  await expect(page.getByRole("button", { name: "登录" })).toBeVisible();
});
