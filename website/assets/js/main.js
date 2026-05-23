// ─────────────────────────────────────────────────────────────────────
//  main.js — interactivity for the architecture site.
//
//  Three responsibilities:
//   1. Pipeline stage tabs (click stage button → show corresponding panel)
//   2. Sidebar nav active highlighting (which section am I reading?)
//   3. Smooth scroll for anchor links (browser default but explicit)
//
//  Plain ES6, no framework, no build step.
// ─────────────────────────────────────────────────────────────────────

(function () {
  'use strict';

  // ── 1. Pipeline stage tabs ───────────────────────────────────────────
  const stageButtons = document.querySelectorAll('.stage-btn');
  const stagePanels = document.querySelectorAll('.stage-panel');

  function showStage(stageName) {
    // Hide all panels
    stagePanels.forEach((panel) => {
      panel.classList.add('hidden');
      panel.classList.remove('active');
    });
    // Remove active from all buttons
    stageButtons.forEach((btn) => btn.classList.remove('active'));

    // Show the requested panel + activate the button
    const targetPanel = document.getElementById('panel-' + stageName);
    if (targetPanel) {
      targetPanel.classList.remove('hidden');
      targetPanel.classList.add('active');
    }
    const targetBtn = document.querySelector('[data-stage="' + stageName + '"]');
    if (targetBtn) {
      targetBtn.classList.add('active');
    }
  }

  // Set "validate" as default open stage on load
  showStage('validate');

  stageButtons.forEach((btn) => {
    btn.addEventListener('click', () => {
      const stage = btn.getAttribute('data-stage');
      showStage(stage);
    });
  });

  // ── 2. Sidebar nav active highlighting via IntersectionObserver ─────
  // Highlights the nav link whose section is currently in view.
  const sections = document.querySelectorAll('main section[id]');
  const navLinks = document.querySelectorAll('.nav-link');

  // Map section id → nav link element
  const navByID = new Map();
  navLinks.forEach((link) => {
    const id = link.getAttribute('href').replace('#', '');
    navByID.set(id, link);
  });

  const observer = new IntersectionObserver(
    (entries) => {
      entries.forEach((entry) => {
        if (entry.isIntersecting) {
          // Clear active from all
          navLinks.forEach((l) => l.classList.remove('active'));
          // Set active on the current one
          const link = navByID.get(entry.target.id);
          if (link) link.classList.add('active');
        }
      });
    },
    {
      // Trigger when the section is in the upper third of the viewport
      rootMargin: '-20% 0px -70% 0px',
      threshold: 0,
    }
  );

  sections.forEach((section) => observer.observe(section));

  // ── 3. Keyboard shortcut: number keys 1-8 to switch pipeline stages ─
  document.addEventListener('keydown', (e) => {
    // Only when no input/textarea is focused
    if (document.activeElement.tagName === 'INPUT' || document.activeElement.tagName === 'TEXTAREA') {
      return;
    }

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
      // Scroll the pipeline section into view if not already
      const pipelineSection = document.getElementById('pipeline');
      if (pipelineSection) {
        const rect = pipelineSection.getBoundingClientRect();
        const isInView = rect.top >= 0 && rect.bottom <= window.innerHeight;
        if (!isInView) {
          pipelineSection.scrollIntoView({ behavior: 'smooth' });
        }
      }
    }
  });
})();
