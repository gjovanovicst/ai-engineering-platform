(() => {
  const root = document.documentElement;
  const theme = document.querySelector(".theme-btn");
  theme?.addEventListener("click", () => {
    const next = root.dataset.theme === "dark" ? "light" : "dark";
    root.dataset.theme = next;
    localStorage.setItem("aip-theme", next);
  });

  const menu = document.querySelector(".menu-btn");
  const nav = document.querySelector(".docs-nav");
  const backdrop = document.querySelector(".nav-backdrop");
  function closeNav() {
    nav?.classList.remove("open");
    if (backdrop) backdrop.hidden = true;
    menu?.setAttribute("aria-expanded", "false");
  }
  menu?.addEventListener("click", () => {
    const open = nav?.classList.toggle("open");
    if (backdrop) backdrop.hidden = !open;
    menu.setAttribute("aria-expanded", String(open));
  });
  backdrop?.addEventListener("click", closeNav);
  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape") closeNav();
  });


  // Highlight the active heading in the "On this page" column.
  const tocLinks = Array.from(document.querySelectorAll(".page-toc a"));
  const tocTargets = tocLinks
    .map((link) => document.getElementById(decodeURIComponent(link.hash.slice(1))))
    .filter(Boolean);
  function updateToc() {
    if (!tocLinks.length || !tocTargets.length) return;
    let active = tocTargets[0];
    for (const heading of tocTargets) {
      if (heading.getBoundingClientRect().top <= 120) active = heading;
      else break;
    }
    tocLinks.forEach((link) => {
      const isActive = link.hash === `#${active.id}`;
      link.classList.toggle("active", isActive);
      if (isActive) link.setAttribute("aria-current", "location");
      else link.removeAttribute("aria-current");
    });
  }
  let tocFrame = 0;
  window.addEventListener("scroll", () => {
    if (tocFrame) return;
    tocFrame = requestAnimationFrame(() => { tocFrame = 0; updateToc(); });
  }, { passive: true });
  updateToc();

  const input = document.getElementById("doc-search");
  const results = document.getElementById("doc-results");
  const inPages = location.pathname.includes("/pages/");
  let index = null;

  function esc(value) {
    const div = document.createElement("div");
    div.textContent = value;
    return div.innerHTML;
  }

  async function loadIndex() {
    if (index) return index;
    const response = await fetch(inPages ? "../search-index.json" : "search-index.json");
    index = await response.json();
    return index;
  }

  function render(rows, query) {
    if (!rows.length) {
      results.innerHTML = `<p class="search-empty">No results for “${esc(query)}”.</p>`;
      return;
    }
    results.innerHTML = rows.slice(0, 9).map((row) => {
      const prefix = inPages ? "" : "pages/";
      const snippet = row.text.toLowerCase().indexOf(query.toLowerCase());
      const text = snippet >= 0 ? row.text.slice(Math.max(0, snippet - 30), snippet + 120) : row.text.slice(0, 130);
      return `<a href="${prefix}${row.slug}.html"><span>${esc(row.title)}</span><small>${esc(text)}…</small></a>`;
    }).join("");
  }

  input?.addEventListener("input", async () => {
    const query = input.value.trim();
    if (query.length < 2) { results.hidden = true; return; }
    const needle = query.toLowerCase();
    const rows = (await loadIndex())
      .map((row) => {
        const title = row.title.toLowerCase();
        const text = row.text.toLowerCase();
        let score = 0;
        if (title === needle) score += 10;
        else if (title.startsWith(needle)) score += 6;
        else if (title.includes(needle)) score += 4;
        if (text.includes(needle)) score += 1;
        return { row, score };
      })
      .filter((item) => item.score > 0)
      .sort((a, b) => b.score - a.score)
      .map((item) => item.row);
    render(rows, query);
    results.hidden = false;
  });
  input?.addEventListener("focus", () => { if (input.value.trim().length >= 2) results.hidden = false; });
  document.addEventListener("click", (event) => {
    if (!results) return;
    if (results.contains(event.target) || input?.contains(event.target)) return;
    results.hidden = true;
  });
  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && results) results.hidden = true;
  });
})();
