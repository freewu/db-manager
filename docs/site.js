/*
 * Behaviour for the introduction site: the language switch and the screenshot
 * carousel.
 *
 * Plain ES5-ish DOM code on purpose — the page is a static document that has to
 * work from a file:// path, from a checkout served by any static server, and on
 * GitHub Pages, with no build step and no dependencies. The copy lives in
 * i18n.js; everything here reads it through `t()`.
 */
(function () {
  'use strict'

  var docs = window.DM_DOCS || { defaultLang: 'en', languages: [], messages: {} }
  var AUTO_ADVANCE = 6000

  var reducedMotion =
    window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches

  function each(selector, fn, scope) {
    Array.prototype.forEach.call((scope || document).querySelectorAll(selector), fn)
  }

  /* ---------------------------------------------------------------- copy -- */

  var lang = docs.defaultLang

  function table(code) {
    return docs.messages[code] || docs.messages[docs.fallbackLang] || {}
  }

  /** Look a key up in the current language, then English, then give it back. */
  function t(key, vars) {
    var value = table(lang)[key]
    if (value === undefined) value = table(docs.fallbackLang)[key]
    if (value === undefined) return key
    if (vars) {
      Object.keys(vars).forEach(function (name) {
        value = value.split('{' + name + '}').join(String(vars[name]))
      })
    }
    return value
  }

  function language(code) {
    var found = null
    docs.languages.forEach(function (entry) {
      if (entry.code === code) found = entry
    })
    return found
  }

  function applyLang() {
    var entry = language(lang)
    document.documentElement.lang = entry ? entry.htmlLang : lang
    document.documentElement.setAttribute('data-docs-lang', lang)
    document.title = t('meta.title')

    each('[data-i18n]', function (el) {
      el.textContent = t(el.getAttribute('data-i18n'))
    })

    // Only two strings carry markup (the `<em>` in the hero headline).
    each('[data-i18n-html]', function (el) {
      el.innerHTML = t(el.getAttribute('data-i18n-html'))
    })

    each('[data-i18n-attr]', function (el) {
      el.getAttribute('data-i18n-attr')
        .split(';')
        .forEach(function (pair) {
          var at = pair.indexOf(':')
          if (at < 1) return
          el.setAttribute(pair.slice(0, at).trim(), t(pair.slice(at + 1).trim()))
        })
    })

    each('.langs [data-lang]', function (button) {
      var on = button.getAttribute('data-lang') === lang
      button.setAttribute('aria-pressed', on ? 'true' : 'false')
      button.classList.toggle('is-active', on)
    })

    each('[data-dot]', function (dot) {
      dot.setAttribute('aria-label', t('carousel.go-to', { n: Number(dot.getAttribute('data-dot')) + 1 }))
    })

    each('[data-thumb]', function (thumb) {
      var title = t('shot.' + thumb.getAttribute('data-thumb-shot') + '.title')
      thumb.setAttribute('aria-label', title)
      thumb.setAttribute('title', title)
      var label = thumb.querySelector('span')
      if (label) label.textContent = title
    })

    renderPlayButton()
    renderLightbox()
  }

  function remember(code) {
    try {
      window.localStorage.setItem(docs.storageKey, code)
    } catch (err) {
      /* file:// or private mode: the choice simply does not outlive the visit */
    }
  }

  function setLang(code) {
    if (!docs.messages[code]) return
    lang = code
    remember(code)
    applyLang()
  }

  function initialLang() {
    // A `?lang=` wins for this visit only; the stored choice is what the next
    // visit starts from; otherwise English, deliberately (no sniffing).
    var asked = null
    try {
      asked = new URLSearchParams(window.location.search).get('lang')
    } catch (err) {
      asked = null
    }
    if (asked && docs.messages[asked]) return asked
    var stored = null
    try {
      stored = window.localStorage.getItem(docs.storageKey)
    } catch (err) {
      stored = null
    }
    return stored && docs.messages[stored] ? stored : docs.defaultLang
  }

  /* ------------------------------------------------------------ carousel -- */

  var carousel = document.querySelector('[data-carousel]')
  var slides = [].slice.call(document.querySelectorAll('[data-slide]'))
  var shots = slides.map(function (slide) {
    var img = slide.querySelector('img')
    return { id: slide.getAttribute('data-shot'), src: img ? img.getAttribute('src') : '' }
  })

  var index = 0
  var playing = true // the user's intent, not the momentary state
  var timer = null
  var inView = true
  var hovering = false
  var focused = false

  function shotKey(i, part) {
    var shot = shots[i]
    return shot ? 'shot.' + shot.id + '.' + part : part
  }

  function show(next) {
    if (!slides.length) return
    index = ((next % slides.length) + slides.length) % slides.length

    var track = carousel.querySelector('[data-slides]')
    if (track) track.style.transform = 'translateX(' + -index * 100 + '%)'

    slides.forEach(function (slide, i) {
      var on = i === index
      slide.classList.toggle('is-active', on)
      // Off-screen slides are neither reachable by Tab nor read out.
      slide.setAttribute('aria-hidden', on ? 'false' : 'true')
      if (on) slide.removeAttribute('inert')
      else slide.setAttribute('inert', '')
    })

    var counter = carousel.querySelector('[data-counter]')
    if (counter) counter.textContent = index + 1 + ' / ' + slides.length

    each('[data-dot]', function (dot, i) {
      dot.classList.toggle('is-active', i === index)
      dot.setAttribute('aria-current', i === index ? 'true' : 'false')
    })
    each('[data-thumb]', function (thumb, i) {
      thumb.classList.toggle('is-active', i === index)
      thumb.setAttribute('aria-current', i === index ? 'true' : 'false')
    })
  }

  function tick() {
    if (document.hidden) return
    show(index + 1)
  }

  function start() {
    if (timer || !playing || !inView || hovering || focused || document.hidden || slides.length < 2) return
    timer = window.setInterval(tick, AUTO_ADVANCE)
  }

  function stop() {
    if (!timer) return
    window.clearInterval(timer)
    timer = null
  }

  function sync() {
    if (playing && inView && !hovering && !focused && !document.hidden) start()
    else stop()
  }

  function setPlaying(on) {
    playing = on
    renderPlayButton()
    sync()
  }

  function renderPlayButton() {
    if (!carousel) return
    var button = carousel.querySelector('[data-carousel-toggle]')
    if (!button) return
    var label = playing ? t('carousel.pause') : t('carousel.play')
    button.classList.toggle('is-paused', !playing)
    button.setAttribute('aria-label', label)
    button.setAttribute('title', label)
  }

  function buildControls() {
    if (!carousel) return

    var dots = carousel.querySelector('[data-dots]')
    var thumbs = carousel.querySelector('[data-thumbs]')

    slides.forEach(function (slide, i) {
      if (dots) {
        var li = document.createElement('li')
        var dot = document.createElement('button')
        dot.type = 'button'
        dot.setAttribute('data-dot', String(i))
        dot.addEventListener('click', function () {
          show(i)
          setPlaying(false)
        })
        li.appendChild(dot)
        dots.appendChild(li)
      }
      if (thumbs) {
        var item = document.createElement('li')
        var thumb = document.createElement('button')
        thumb.type = 'button'
        thumb.setAttribute('data-thumb', String(i))
        thumb.setAttribute('data-thumb-shot', shots[i].id)
        var image = document.createElement('img')
        image.setAttribute('src', shots[i].src)
        image.setAttribute('alt', '')
        image.setAttribute('loading', 'lazy')
        image.setAttribute('decoding', 'async')
        var caption = document.createElement('span')
        thumb.appendChild(image)
        thumb.appendChild(caption)
        thumb.addEventListener('click', function () {
          show(i)
          setPlaying(false)
        })
        item.appendChild(thumb)
        thumbs.appendChild(item)
      }
    })

    each('[data-lightbox]', function (button) {
      button.addEventListener('click', function () {
        var slide = button.closest('[data-slide]')
        var link = button.getAttribute('data-shot-link')
        var at = slide ? slides.indexOf(slide) : -1
        if (at < 0 && link) {
          shots.forEach(function (shot, i) {
            if (shot.id === link) at = i
          })
        }
        if (at >= 0) openLightbox(at)
      })
    })

    var prev = carousel.querySelector('[data-carousel-prev]')
    var next = carousel.querySelector('[data-carousel-next]')
    var toggle = carousel.querySelector('[data-carousel-toggle]')
    if (prev)
      prev.addEventListener('click', function () {
        show(index - 1)
        setPlaying(false)
      })
    if (next)
      next.addEventListener('click', function () {
        show(index + 1)
        setPlaying(false)
      })
    if (toggle)
      toggle.addEventListener('click', function () {
        setPlaying(!playing)
      })

    carousel.addEventListener('mouseenter', function () {
      hovering = true
      sync()
    })
    carousel.addEventListener('mouseleave', function () {
      hovering = false
      sync()
    })
    carousel.addEventListener('focusin', function () {
      focused = true
      sync()
    })
    carousel.addEventListener('focusout', function () {
      focused = false
      sync()
    })
    carousel.addEventListener('keydown', function (event) {
      if (event.key === 'ArrowLeft') {
        show(index - 1)
        setPlaying(false)
        event.preventDefault()
      } else if (event.key === 'ArrowRight') {
        show(index + 1)
        setPlaying(false)
        event.preventDefault()
      }
    })

    if (typeof window.IntersectionObserver === 'function') {
      new window.IntersectionObserver(
        function (entries) {
          inView = entries[0].isIntersecting
          sync()
        },
        { threshold: 0.15 },
      ).observe(carousel)
    }

    document.addEventListener('visibilitychange', sync)
    show(0)
  }

  /* ------------------------------------------------------------ lightbox -- */

  var box = document.querySelector('[data-lightbox-box]')
  var boxIndex = -1
  var lastFocus = null

  function moveLightbox(step) {
    if (boxIndex < 0 || !shots.length) return
    boxIndex = ((boxIndex + step) % shots.length + shots.length) % shots.length
    renderLightbox()
  }

  function renderLightbox() {
    if (!box || boxIndex < 0) return
    var image = box.querySelector('[data-lb-image]')
    var title = box.querySelector('[data-lb-title]')
    var desc = box.querySelector('[data-lb-desc]')
    if (image) {
      image.setAttribute('src', shots[boxIndex].src)
      image.setAttribute('alt', t(shotKey(boxIndex, 'desc')))
    }
    if (title) title.textContent = t(shotKey(boxIndex, 'title'))
    if (desc) desc.textContent = t(shotKey(boxIndex, 'desc'))
  }

  function openLightbox(at) {
    if (!box) return
    boxIndex = at
    lastFocus = document.activeElement
    renderLightbox()
    box.hidden = false
    document.body.classList.add('is-locked')
    var close = box.querySelector('[data-lb-close]')
    if (close) close.focus()
  }

  function closeLightbox() {
    if (!box || box.hidden) return
    box.hidden = true
    boxIndex = -1
    document.body.classList.remove('is-locked')
    if (lastFocus && lastFocus.focus) lastFocus.focus()
    lastFocus = null
  }

  function wireLightbox() {
    if (!box) return
    var close = box.querySelector('[data-lb-close]')
    var prev = box.querySelector('[data-lb-prev]')
    var next = box.querySelector('[data-lb-next]')
    if (close) close.addEventListener('click', closeLightbox)
    if (prev) prev.addEventListener('click', function () { moveLightbox(-1) })
    if (next) next.addEventListener('click', function () { moveLightbox(1) })
    box.addEventListener('click', function (event) {
      if (event.target === box) closeLightbox()
    })
    document.addEventListener('keydown', function (event) {
      if (box.hidden) return
      if (event.key === 'Escape') {
        closeLightbox()
      } else if (event.key === 'ArrowLeft') {
        moveLightbox(-1)
      } else if (event.key === 'ArrowRight') {
        moveLightbox(1)
      } else {
        return
      }
      event.preventDefault()
    })
  }

  /* ------------------------------------------------------------- version -- */

  // The product version lives in wails.json, the single source of truth for it.
  // Read at run time so the site never needs regenerating for a release; when
  // the file is not reachable (file://, or a host that only serves docs/) the
  // badge is simply left out instead of showing a stale number.
  function loadVersion() {
    var badge = document.querySelector('[data-version]')
    if (!badge) return
    if (window.location.protocol === 'file:' || typeof window.fetch !== 'function') return
    window
      .fetch('../wails.json', { cache: 'no-store' })
      .then(function (response) {
        return response.ok ? response.json() : null
      })
      .then(function (data) {
        var version = data && data.info && data.info.productVersion
        if (!version) return
        badge.textContent = 'v' + version
        badge.hidden = false
      })
      .catch(function () {
        /* nothing to show, and nothing to say about it */
      })
  }

  /* ---------------------------------------------------------------- boot -- */

  function boot() {
    if (reducedMotion) document.documentElement.setAttribute('data-reduced-motion', '')

    each('.langs [data-lang]', function (button) {
      button.addEventListener('click', function () {
        setLang(button.getAttribute('data-lang'))
      })
    })

    buildControls()
    wireLightbox()
    lang = initialLang()
    applyLang()
    loadVersion()
  }

  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', boot)
  else boot()
})()
