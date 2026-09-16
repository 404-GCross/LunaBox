/* eslint-disable antfu/no-top-level-await -- Standalone browser regression runner. */
import assert from "node:assert/strict";
import { mkdtemp } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import process from "node:process";

const { chromium } = await import(
  process.env.PLAYWRIGHT_MODULE ?? "playwright",
);
const browser = await chromium.launch({ channel: "chrome", headless: true });
const artifacts = await mkdtemp(join(tmpdir(), "lunabox-desktop-shell-"));
const failures = [];
try {
  for (const viewport of [
    { width: 1280, height: 800 },
    { width: 540, height: 760 },
  ]) {
    const page = await browser.newPage({ viewport });
    page.setDefaultTimeout(8000);
    page.on("pageerror", error => failures.push(error.message));
    await page.goto(
      `${process.env.TEST_BASE_URL ?? "http://127.0.0.1:9245"}/tests/desktopShell.html`,
    );
    const drawer = page.getByRole("dialog", {
      name: "Test drawer",
      exact: true,
    });
    const open = page.getByTestId("open");
    const control = page.getByTestId("window-control");
    await open.click();
    await drawer.waitFor({ state: "visible" });
    assert.equal(await page.evaluate(() => window.scrollX), 0);
    await page.waitForTimeout(100);
    assert.equal(await page.evaluate(() => window.scrollX), 0);
    await page.waitForTimeout(350);
    assert.equal(
      await page.getByTestId("content").evaluate(element => element.inert),
      true,
    );
    assert.equal(
      await control.evaluate(element => !!element.closest("[inert]")),
      false,
    );
    await control.click();
    assert.match(await control.textContent(), /1$/);
    assert.equal(await drawer.isVisible(), true);
    await page.mouse.click(viewport.width - 15, 12);
    assert.equal(await drawer.isVisible(), true);
    await page.getByTestId("height").click();
    assert.equal(Math.round((await drawer.boundingBox()).y), 52);
    await drawer.getByRole("textbox").click();
    const input = await drawer.getByRole("textbox").boundingBox();
    await page.mouse.move(input.x + 10, input.y + 10);
    await page.mouse.down();
    await page.mouse.move(10, 100);
    await page.mouse.up();
    assert.equal(await drawer.isVisible(), true);
    await drawer.getByRole("button", { name: "One", exact: true }).click();
    await page.getByRole("option", { name: "Two", exact: true }).click();
    assert.equal(await drawer.isVisible(), true);
    await drawer.getByRole("button", { name: "Two", exact: true }).click();
    assert.equal(
      await control.evaluate(element => !!element.closest("[inert]")),
      false,
    );
    await page.keyboard.press("Escape");
    assert.equal(await page.getByRole("listbox").count(), 0);
    assert.equal(await drawer.isVisible(), true);
    await page.getByTestId("nested").click();
    const nested = page.getByRole("dialog", {
      name: "Nested drawer",
      exact: true,
    });
    await nested.waitFor({ state: "visible" });
    await page.keyboard.press("Escape");
    await nested.waitFor({ state: "detached" });
    assert.equal(await drawer.isVisible(), true);
    await drawer.getByRole("textbox").click();
    for (let index = 0; index < 8; index++) {
      await page.keyboard.press("Tab");
      assert.equal(
        await drawer.evaluate(element =>
          element.contains(document.activeElement),
        ),
        true,
      );
    }
    await page.screenshot({
      path: join(artifacts, `${viewport.width}-drawer.png`),
    });
    await page.evaluate(() => document.documentElement.classList.add("dark"));
    await page.screenshot({
      path: join(artifacts, `${viewport.width}-dark.png`),
    });
    await page.evaluate(() => {
      document.documentElement.style.fontSize = "20px";
    });
    assert.equal(Math.round((await drawer.boundingBox()).y), 52);
    await control.click();
    assert.equal(await drawer.isVisible(), true);
    await page.mouse.click(10, 100);
    await drawer.waitFor({ state: "detached" });
    assert.equal(
      await page.getByTestId("content").evaluate(element => element.inert),
      false,
    );
    assert.equal(
      await open.evaluate(element => element === document.activeElement),
      true,
    );
    await open.click();
    await drawer.waitFor({ state: "visible" });
    await page.keyboard.press("Escape");
    await drawer.waitFor({ state: "detached" });
    await open.click();
    await drawer.getByRole("button", { name: "Close", exact: true }).click();
    await drawer.waitFor({ state: "detached" });
    await page.close();
  }
  assert.deepEqual(failures, []);
  process.stdout.write(`Desktop shell interactions passed. Screenshots: ${artifacts}\n`);
}
finally {
  await browser.close();
}
