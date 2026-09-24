// Service worker: держит ресурсы игры в Cache Storage.
//
// Зачем он вообще нужен. Полагаться на обычный HTTP-кеш здесь нельзя: у Chrome
// дисковый кеш отказывается хранить записи крупнее примерно 1/8 своего размера,
// а WAVE.DAN весит 115 МБ — он вылетает всегда, и каждый запуск тянул бы его
// заново. Cache Storage живёт по квоте origin-а (здесь ~4 ГБ), лимита на размер
// записи у него нет, и заполненность видно через navigator.storage.estimate().
//
// Игра ничего про это не знает: fetch из wasm проходит через worker так же, как
// любой другой, поэтому загрузчик в Go остался нетронутым.

// Имя кеша привязано к версии набора: публикация /v2/ отправит /v1/ целиком в
// мусор одной строкой в activate.
const CACHE = "robinson-v1";

// Кешируем только ресурсы: они лежат под версионным префиксом и по своему
// адресу неизменны. Оболочка (wasm, html) намеренно мимо — она пересобирается
// на каждый деплой и должна приезжать свежей.
const VERSIONED = /^\/v\d+\//;

// Манифест — единственный изменяемый файл внутри версии: через него объявляется
// новая раскладка, так что его кешировать нельзя.
const MANIFEST = /^\/v\d+\/manifest\.json$/;

self.addEventListener("install", () => {
  // Ждать закрытия старых вкладок незачем: старый worker кеш не испортит.
  self.skipWaiting();
});

self.addEventListener("activate", (event) => {
  event.waitUntil((async () => {
    const names = await caches.keys();
    await Promise.all(
      names.filter((n) => n !== CACHE).map((n) => caches.delete(n)),
    );
    // Берём под контроль уже открытую страницу, иначе на первой загрузке
    // worker простоял бы без дела и ресурсы прошли бы мимо кеша.
    await self.clients.claim();
  })());
});

self.addEventListener("fetch", (event) => {
  const req = event.request;
  if (req.method !== "GET") return;

  const url = new URL(req.url);
  if (url.origin !== self.location.origin) return;
  if (!VERSIONED.test(url.pathname) || MANIFEST.test(url.pathname)) return;

  event.respondWith((async () => {
    const cache = await caches.open(CACHE);
    const hit = await cache.match(req);
    if (hit) return hit;

    const resp = await fetch(req);
    // Класть в кеш только удачные ответы: сохранённая 404 пережила бы починку
    // хоста и осталась бы битой навсегда.
    if (resp.ok) {
      // Без await: пусть игра получит байты, не дожидаясь записи на диск.
      // Переполнение квоты не должно ронять загрузку — тогда просто не
      // закешируется.
      cache.put(req, resp.clone()).catch(() => {});
    }
    return resp;
  })());
});
