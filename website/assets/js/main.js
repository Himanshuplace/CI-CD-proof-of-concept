// ═══════════════════════════════════════════════════════════════════════
//  main.js — interactivity for the CI/CD Field Manual
//
//  Five responsibilities:
//   1. Theme switcher (light / sepia / dark) with localStorage + system pref
//   2. Reading progress bar (top of page)
//   3. Pipeline stage tabs (click + keyboard shortcuts)
//   4. Scroll-triggered figure reveals
//   5. Smooth anchor navigation (topbar-aware)
//
//  Plain ES6, no framework, no build step.
//  Theme application happens in an inline <script> in <head> BEFORE first
//  paint to avoid the flash-of-wrong-theme on load. This file owns the
//  rest of the theme lifecycle: button clicks, syncing the UI, persisting.
// ═══════════════════════════════════════════════════════════════════════

(function () {
  'use strict';

  // ── 1. Theme switcher ────────────────────────────────────────────────

  const THEMES = ['light', 'sepia', 'dark'];
  const themeButtons = document.querySelectorAll('.theme-switch button[data-theme-set]');

  function currentTheme() {
    // Explicit theme on <html> wins. Otherwise consult system preference.
    const explicit = document.documentElement.getAttribute('data-theme');
    if (THEMES.includes(explicit)) return explicit;
    if (window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches) {
      return 'dark';
    }
    return 'light';
  }

  function applyTheme(theme) {
    if (!THEMES.includes(theme)) return;
    document.documentElement.setAttribute('data-theme', theme);
    try { localStorage.setItem('theme', theme); } catch (e) { /* private mode */ }
    syncThemeButtons(theme);
  }

  function syncThemeButtons(active) {
    themeButtons.forEach((btn) => {
      const isActive = btn.getAttribute('data-theme-set') === active;
      btn.classList.toggle('active', isActive);
      btn.setAttribute('aria-pressed', isActive ? 'true' : 'false');
    });
  }

  themeButtons.forEach((btn) => {
    btn.addEventListener('click', () => {
      applyTheme(btn.getAttribute('data-theme-set'));
    });
  });

  // Initial sync — reflect whatever theme is active right now
  syncThemeButtons(currentTheme());

  // If the user hasn't explicitly chosen, follow system changes live
  if (window.matchMedia) {
    const mql = window.matchMedia('(prefers-color-scheme: dark)');
    mql.addEventListener?.('change', (e) => {
      const userChose = (() => { try { return !!localStorage.getItem('theme'); } catch { return false; } })();
      if (userChose) return; // user override wins
      document.documentElement.removeAttribute('data-theme');
      syncThemeButtons(e.matches ? 'dark' : 'light');
    });
  }

  // Keyboard shortcut: T cycles themes
  document.addEventListener('keydown', (e) => {
    if (e.metaKey || e.ctrlKey || e.altKey) return;
    const tag = document.activeElement.tagName;
    if (tag === 'INPUT' || tag === 'TEXTAREA') return;

    if (e.key === 't' || e.key === 'T') {
      const idx = THEMES.indexOf(currentTheme());
      const next = THEMES[(idx + 1) % THEMES.length];
      applyTheme(next);
    }
  });

  // ── 2. Reading progress bar ──────────────────────────────────────────
  const progressEl = document.getElementById('progress');

  function updateProgress() {
    const doc = document.documentElement;
    const scrollTop = doc.scrollTop || document.body.scrollTop;
    const scrollHeight = doc.scrollHeight - doc.clientHeight;
    if (scrollHeight <= 0) return;
    const percent = Math.min(100, (scrollTop / scrollHeight) * 100);
    if (progressEl) progressEl.style.width = percent + '%';
  }

  let rafScheduled = false;
  window.addEventListener('scroll', () => {
    if (rafScheduled) return;
    rafScheduled = true;
    requestAnimationFrame(() => {
      updateProgress();
      rafScheduled = false;
    });
  }, { passive: true });
  updateProgress();

  // ── 3. Pipeline stage tabs ───────────────────────────────────────────
  const stageButtons = document.querySelectorAll('.stage-tile');
  const stagePanels = document.querySelectorAll('.stage-panel');

  function showStage(stageName) {
    stagePanels.forEach((panel) => panel.classList.add('hidden'));
    stageButtons.forEach((btn) => btn.classList.remove('active'));

    const panel = document.getElementById('panel-' + stageName);
    if (panel) panel.classList.remove('hidden');

    const btn = document.querySelector('.stage-tile[data-stage="' + stageName + '"]');
    if (btn) btn.classList.add('active');
  }

  stageButtons.forEach((btn) => {
    btn.addEventListener('click', () => {
      showStage(btn.getAttribute('data-stage'));
    });
  });

  document.addEventListener('keydown', (e) => {
    if (e.metaKey || e.ctrlKey || e.altKey) return;
    const tag = document.activeElement.tagName;
    if (tag === 'INPUT' || tag === 'TEXTAREA') return;

    const stageMap = {
      '1': 'validate', '2': 'test', '3': 'security', '4': 'build',
      '5': 'package',  '6': 'deploy', '7': 'observe', '8': 'agentic',
    };

    if (stageMap[e.key]) {
      e.preventDefault();
      showStage(stageMap[e.key]);
      const rail = document.querySelector('.pipeline-rail');
      if (rail) {
        const rect = rail.getBoundingClientRect();
        const inView = rect.top >= 0 && rect.bottom <= window.innerHeight + 200;
        if (!inView) rail.scrollIntoView({ behavior: 'smooth', block: 'center' });
      }
    }
  });

  const initialStage =
    document.querySelector('.stage-tile.active')?.getAttribute('data-stage') || 'validate';
  showStage(initialStage);

  // ── 4. Scroll-triggered reveals ──────────────────────────────────────
  const revealEls = document.querySelectorAll(
    'figure.diagram, .pullquote, .tl-item, .card-row > div, .numbered > li'
  );
  revealEls.forEach((el) => el.classList.add('reveal'));

  if ('IntersectionObserver' in window) {
    const obs = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            entry.target.classList.add('in');
            obs.unobserve(entry.target);
          }
        });
      },
      { rootMargin: '0px 0px -80px 0px', threshold: 0.05 }
    );
    revealEls.forEach((el) => obs.observe(el));
  } else {
    revealEls.forEach((el) => el.classList.add('in'));
  }

  // ── 5. Smooth anchor nav with topbar offset ──────────────────────────
  document.querySelectorAll('a[href^="#"]').forEach((link) => {
    link.addEventListener('click', (e) => {
      const href = link.getAttribute('href');
      if (href === '#' || href.length < 2) return;
      const target = document.querySelector(href);
      if (!target) return;
      e.preventDefault();
      const topbarH = document.querySelector('.topbar')?.offsetHeight || 0;
      const y = target.getBoundingClientRect().top + window.pageYOffset - topbarH - 16;
      window.scrollTo({ top: y, behavior: 'smooth' });
      history.pushState(null, '', href);
    });
  });
})();
