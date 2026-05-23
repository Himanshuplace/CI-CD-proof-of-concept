// ═══════════════════════════════════════════════════════════════════════
//  main.js — interactivity for the CI/CD Field Manual
//
//  Four responsibilities:
//   1. Reading progress bar (top of page)
//   2. Pipeline stage tabs (click + keyboard shortcuts)
//   3. Scroll-triggered diagram reveals
//   4. Smooth anchor navigation
//
//  Plain ES6, no framework, no build step.
// ═══════════════════════════════════════════════════════════════════════

(function () {
  'use strict';

  // ── 1. Reading progress bar ──────────────────────────────────────────
  const progressEl = document.getElementById('progress');

  function updateProgress() {
    const doc = document.documentElement;
    const scrollTop = doc.scrollTop || document.body.scrollTop;
    const scrollHeight = doc.scrollHeight - doc.clientHeight;
    if (scrollHeight <= 0) return;
    const percent = Math.min(100, (scrollTop / scrollHeight) * 100);
    if (progressEl) progressEl.style.width = percent + '%';
  }

  // rAF-throttled scroll listener: avoids hammering layout on every pixel
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

  // ── 2. Pipeline stage tabs ───────────────────────────────────────────
  const stageButtons = document.querySelectorAll('.stage-tile');
  const stagePanels = document.querySelectorAll('.stage-panel');

  function showStage(stageName) {
    stagePanels.forEach((panel) => {
      panel.classList.add('hidden');
    });
    stageButtons.forEach((btn) => btn.classList.remove('active'));

    const targetPanel = document.getElementById('panel-' + stageName);
    if (targetPanel) targetPanel.classList.remove('hidden');

    const targetBtn = document.querySelector('.stage-tile[data-stage="' + stageName + '"]');
    if (targetBtn) targetBtn.classList.add('active');
  }

  stageButtons.forEach((btn) => {
    btn.addEventListener('click', () => {
      const stage = btn.getAttribute('data-stage');
      showStage(stage);
    });
  });

  // Keyboard shortcuts 1-8 for pipeline stages
  document.addEventListener('keydown', (e) => {
    if (e.metaKey || e.ctrlKey || e.altKey) return;
    const tag = document.activeElement.tagName;
    if (tag === 'INPUT' || tag === 'TEXTAREA') return;

    const stageMap = {
      '1': 'validate',
      '2': 'test',
      '3': 'security',
      '4': 'build',
      '5': 'package',
      '6': 'deploy',
      '7': 'observe',
      '8': 'agentic',
    };

    if (stageMap[e.key]) {
      e.preventDefault();
      showStage(stageMap[e.key]);

      const pipelineSection = document.getElementById('ch3');
      if (pipelineSection) {
        const rect = pipelineSection.getBoundingClientRect();
        const isInView = rect.top >= 0 && rect.bottom <= window.innerHeight + 200;
        if (!isInView) {
          // jump to the pipeline rail rather than the chapter top
          const rail = document.querySelector('.pipeline-rail');
          if (rail) rail.scrollIntoView({ behavior: 'smooth', block: 'center' });
        }
      }
    }
  });

  // ── 3. Scroll-triggered reveals ──────────────────────────────────────
  // Figures fade in as they enter the viewport. Subtle — doesn't fight
  // with the reading flow, just adds a sense that the page is alive.
  const revealEls = document.querySelectorAll('figure.diagram, .pullquote, .tl-item, .card-row > div, .numbered > li');

  // Add the .reveal class via JS so users without JS still see content normally
  revealEls.forEach((el) => el.classList.add('reveal'));

  if ('IntersectionObserver' in window) {
    const revealObs = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            entry.target.classList.add('in');
            revealObs.unobserve(entry.target);
          }
        });
      },
      { rootMargin: '0px 0px -80px 0px', threshold: 0.05 }
    );
    revealEls.forEach((el) => revealObs.observe(el));
  } else {
    // No IntersectionObserver → show everything immediately
    revealEls.forEach((el) => el.classList.add('in'));
  }

  // ── 4. Smooth anchor navigation ──────────────────────────────────────
  // Browser default already does this with CSS scroll-behavior, but adding
  // a tiny offset for the sticky topbar makes the result feel cleaner.
  document.querySelectorAll('a[href^="#"]').forEach((link) => {
    link.addEventListener('click', (e) => {
      const href = link.getAttribute('href');
      if (href === '#' || href.length < 2) return;
      const target = document.querySelector(href);
      if (!target) return;
      e.preventDefault();

      const topbarHeight = document.querySelector('.topbar')?.offsetHeight || 0;
      const y = target.getBoundingClientRect().top + window.pageYOffset - topbarHeight - 16;
      window.scrollTo({ top: y, behavior: 'smooth' });
      // Update the hash without re-triggering scroll
      history.pushState(null, '', href);
    });
  });

  // ── 5. Default open stage on load ────────────────────────────────────
  // The HTML already marks "validate" as .active in the rail, but we
  // also make sure the panel state matches in case the HTML drifts.
  const initialStage = document.querySelector('.stage-tile.active')?.getAttribute('data-stage') || 'validate';
  showStage(initialStage);

})();
