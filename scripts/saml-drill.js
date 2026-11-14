#!/usr/bin/env node
// SAML browser drill against Keycloak. Requires Playwright + Chromium.
const { chromium } = require("playwright");
const api = process.env.AIP_API_URL || "http://localhost:18080";
const web = process.env.AIP_WEB_URL || "http://localhost:3000";
const username = process.env.AIP_E2E_EMAIL || "demo";
const password = process.env.AIP_E2E_PASSWORD || "correct horse battery";
(async () => {
  const browser = await chromium.launch({ executablePath: process.env.AIP_CHROMIUM || undefined, headless: true, args: ["--no-sandbox"] });
  const page = await browser.newPage();
  await page.goto(`${api}/saml/login`, { waitUntil: "domcontentloaded" });
  await page.fill("#username", username);
  await page.fill("#password", password);
  await page.click("#kc-login");
  await page.waitForURL(new RegExp(`${web.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}/projects`), { timeout: 20000 });
  await page.waitForTimeout(1500);
  const text = await page.locator("body").innerText();
  const loggedIn = !text.includes("Sign in to workspace");
  console.log(`url=${page.url()} logged_in=${loggedIn} has_projects=${text.includes("Novasoft CRM")}`);
  await browser.close();
  if (!loggedIn) process.exit(1);
})().catch((err) => { console.error(err); process.exit(1); });
