// Applies the saved or system theme before the first paint, so pages never flash
// in the wrong theme. A file rather than an inline script, which the CSP blocks.
(function () {
  var pref = 'system'
  try {
    pref = localStorage.getItem('pbom-theme') || 'system'
  } catch (e) {}
  var dark = pref === 'dark' || (pref !== 'light' && window.matchMedia('(prefers-color-scheme: dark)').matches)
  document.documentElement.dataset.theme = dark ? 'dark' : 'light'
})()
