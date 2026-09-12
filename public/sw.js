const CACHE = 'foodie-shell-v6';
const LAZY = ['/foodie-3d.js'];
const SHELL = ['/', '/language.js', '/style.css', '/app.js', '/legacy-units.js', '/icon.svg?v=foodie-122', '/icon-192.png?v=foodie-122', '/icon-512.png?v=foodie-122', '/apple-touch-icon.png?v=foodie-122', '/manifest.webmanifest'];
self.addEventListener('install', event => { event.waitUntil(caches.open(CACHE).then(cache => cache.addAll(SHELL)).then(() => self.skipWaiting())); });
self.addEventListener('activate', event => { event.waitUntil(caches.keys().then(keys => Promise.all(keys.filter(key => key !== CACHE).map(key => caches.delete(key)))).then(() => self.clients.claim())); });
self.addEventListener('fetch', event => {
  const url = new URL(event.request.url);
  if (event.request.method !== 'GET' || url.origin !== self.location.origin || ![...SHELL, ...LAZY].some(path => path.split('?')[0] === url.pathname)) return;
  event.respondWith(fetch(event.request).then(response => { if (response.ok) { const copy = response.clone(); caches.open(CACHE).then(cache => cache.put(event.request, copy)); } return response; }).catch(() => caches.match(event.request)));
});
