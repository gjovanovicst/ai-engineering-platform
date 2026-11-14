(() => {
  const root = document.documentElement;
  const toggle = document.querySelector(".theme-toggle");
  const stored = localStorage.getItem("aip-theme");
  const systemLight = window.matchMedia("(prefers-color-scheme: light)");
  if (stored === "light" || stored === "dark") {
    root.dataset.theme = stored;
  } else if (systemLight.matches) {
    root.dataset.theme = "light";
  }
  const themeColor = document.querySelector('meta[name="theme-color"]');
  if (themeColor) themeColor.setAttribute("content", root.dataset.theme === "light" ? "#f6f7fb" : "#080b12");
  function applyTheme(next) {
    root.dataset.theme = next;
    localStorage.setItem("aip-theme", next);
    if (themeColor) themeColor.setAttribute("content", next === "dark" ? "#080b12" : "#f6f7fb");
  }
  toggle?.addEventListener("click", () => {
    applyTheme(root.dataset.theme === "dark" ? "light" : "dark");
  });
  systemLight.addEventListener?.("change", (event) => {
    if (!localStorage.getItem("aip-theme")) {
      root.dataset.theme = event.matches ? "light" : "dark";
    }
  });

  const navToggle = document.querySelector(".nav-toggle");
  const links = document.querySelector(".nav-links");
  navToggle?.addEventListener("click", () => {
    const open = links.classList.toggle("open");
    navToggle.setAttribute("aria-expanded", String(open));
  });
  links?.querySelectorAll("a").forEach((a) => a.addEventListener("click", () => {
    links.classList.remove("open");
    navToggle?.setAttribute("aria-expanded", "false");
  }));

  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && links?.classList.contains("open")) {
      links.classList.remove("open");
      navToggle?.setAttribute("aria-expanded", "false");
      navToggle?.focus();
    }
  });
  document.addEventListener("click", (event) => {
    if (!links?.classList.contains("open")) return;
    if (links.contains(event.target) || navToggle?.contains(event.target)) return;
    links.classList.remove("open");
    navToggle?.setAttribute("aria-expanded", "false");
  });

  const header = document.querySelector(".nav-wrap");
  const onScroll = () => header?.classList.toggle("scrolled", window.scrollY > 8);
  onScroll();
  window.addEventListener("scroll", onScroll, { passive: true });

  document.querySelectorAll("[data-copy]").forEach((button) => {
    button.addEventListener("click", async () => {
      const target = document.getElementById(button.dataset.copy);
      if (!target) return;
      try {
        await navigator.clipboard.writeText(target.innerText);
        const old = button.textContent;
        button.textContent = "Copied";
        setTimeout(() => { button.textContent = old; }, 1500);
      } catch {
        button.textContent = "Copy failed";
      }
    });
  });


  const tabs = Array.from(document.querySelectorAll(".tour-tab"));
  const panels = Array.from(document.querySelectorAll(".tour-panel"));
  function activateTour(name, focus = false) {
    tabs.forEach((tab) => {
      const active = tab.dataset.tab === name;
      tab.classList.toggle("active", active);
      tab.setAttribute("aria-selected", String(active));
      tab.tabIndex = active ? 0 : -1;
      if (active && focus) tab.focus();
    });
    panels.forEach((panel) => {
      const active = panel.dataset.panel === name;
      panel.classList.toggle("active", active);
      panel.hidden = !active;
    });
  }
  if (tabs.length) activateTour("overview");
  tabs.forEach((tab, index) => {
    tab.addEventListener("click", () => activateTour(tab.dataset.tab));
    tab.addEventListener("keydown", (event) => {
      if (!["ArrowRight", "ArrowDown", "ArrowLeft", "ArrowUp"].includes(event.key)) return;
      event.preventDefault();
      const next = ["ArrowRight", "ArrowDown"].includes(event.key) ? (index + 1) % tabs.length : (index - 1 + tabs.length) % tabs.length;
      activateTour(tabs[next].dataset.tab, true);
    });
  });


  // Lightbox for product screenshots.
  const lightbox = document.getElementById("lightbox");
  const lightboxImg = document.getElementById("lightbox-img");
  const lightboxCaption = document.getElementById("lightbox-caption");
  const lightboxClose = lightbox?.querySelector(".lightbox-close");
  const lightboxPrev = lightbox?.querySelector(".lightbox-prev");
  const lightboxNext = lightbox?.querySelector(".lightbox-next");
  const gallery = Array.from(document.querySelectorAll("a.tour-frame, a.shot-card, a.shot-img"));
  let lightboxIndex = -1;
  let lightboxTrigger = null;

  function showSlide(index) {
    if (!gallery.length) return;
    lightboxIndex = (index + gallery.length) % gallery.length;
    const link = gallery[lightboxIndex];
    const img = link.querySelector("img");
    if (!img) return;
    lightboxImg.src = link.getAttribute("href");
    lightboxImg.alt = img.alt;
    lightboxCaption.textContent = img.alt;
    lightboxPrev.hidden = lightboxNext.hidden = gallery.length < 2;
  }

  function closeLightbox() {
    if (!lightbox || lightbox.hidden) return;
    lightbox.hidden = true;
    document.body.classList.remove("lightbox-open");
    lightboxImg.src = "";
    if (lightboxTrigger) lightboxTrigger.focus();
    lightboxTrigger = null;
    lightboxIndex = -1;
  }

  function openLightbox(index) {
    if (!lightbox) return;
    lightboxTrigger = gallery[index];
    showSlide(index);
    lightbox.hidden = false;
    document.body.classList.add("lightbox-open");
    lightboxClose.focus();
  }

  gallery.forEach((link, index) => {
    link.addEventListener("click", (event) => {
      if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
      event.preventDefault();
      openLightbox(index);
    });
  });

  lightboxClose?.addEventListener("click", closeLightbox);
  lightboxPrev?.addEventListener("click", () => showSlide(lightboxIndex - 1));
  lightboxNext?.addEventListener("click", () => showSlide(lightboxIndex + 1));
  lightbox?.addEventListener("click", (event) => {
    if (event.target === lightbox) closeLightbox();
  });

  document.addEventListener("keydown", (event) => {
    if (!lightbox || lightbox.hidden) return;
    if (event.key === "Escape") { closeLightbox(); return; }
    if (event.key === "ArrowLeft") { showSlide(lightboxIndex - 1); return; }
    if (event.key === "ArrowRight") { showSlide(lightboxIndex + 1); return; }
    if (event.key !== "Tab") return;
    const focusables = [lightboxClose, lightboxPrev, lightboxNext].filter((el) => el && !el.hidden);
    if (!focusables.length) return;
    const first = focusables[0];
    const last = focusables[focusables.length - 1];
    if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
    else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
  });

  const sectionLinks = Array.from(document.querySelectorAll('.nav-links a[href^="#"]'));
  const sections = sectionLinks
    .map((link) => document.querySelector(link.getAttribute("href")))
    .filter(Boolean);
  if ("IntersectionObserver" in window && sections.length) {
    const spy = new IntersectionObserver((entries) => {
      entries.forEach((entry) => {
        if (!entry.isIntersecting) return;
        sectionLinks.forEach((link) => {
          const active = link.getAttribute("href") === `#${entry.target.id}`;
          link.classList.toggle("active", active);
          if (active) link.setAttribute("aria-current", "page");
          else link.removeAttribute("aria-current");
        });
      });
    }, { rootMargin: "-45% 0px -50%" });
    sections.forEach((section) => spy.observe(section));
  }

  const reveals = document.querySelectorAll(".reveal");
  if (!("IntersectionObserver" in window)) {
    reveals.forEach((el) => el.classList.add("visible"));
    return;
  }
  const observer = new IntersectionObserver((entries) => {
    entries.forEach((entry) => {
      if (entry.isIntersecting) {
        entry.target.classList.add("visible");
        observer.unobserve(entry.target);
      }
    });
  }, { threshold: 0.12 });
  reveals.forEach((el) => observer.observe(el));
})();
