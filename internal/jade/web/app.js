const el = id => document.getElementById(id);
const form = el('form');
const input = el('url');
const submit = el('submit');
const result = el('result');
const share = el('share');
const copy = el('copy');
const check = el('check');
const preview = el('preview');
let version = 0;

function status(id, message, error = false) {
  const field = el(id);
  field.textContent = message;
  field.classList.toggle('error', error);
}

function resetResult() {
  version++;
  submit.disabled = false;
  result.hidden = true;
  el('media').replaceChildren();
  el('pages').replaceChildren();
  input.removeAttribute('aria-invalid');
  status('status', '');
}

async function get(path, source) {
  const response = await fetch(path + '?' + new URLSearchParams({ url: source }));
  const data = await response.json();
  if (!response.ok) throw new Error(data.error || 'The request failed.');
  return data;
}

input.addEventListener('input', resetResult);
input.addEventListener('paste', event => {
  const pasted = event.clipboardData?.getData('text').trim();
  if (!pasted || pasted.length > input.maxLength || /\s/.test(pasted)) return;
  event.preventDefault();
  input.value = pasted;
  form.requestSubmit();
});

form.addEventListener('submit', async event => {
  event.preventDefault();
  resetResult();
  const current = version;
  const source = input.value.trim();
  submit.disabled = true;
  status('status', 'Creating link…');
  try {
    const data = await get('/api/link', source);
    if (current !== version) return;
    share.value = data.share_url;
    copy.textContent = 'Copy link';
    check.disabled = false;
    check.textContent = 'Preview media';
    check.setAttribute('aria-expanded', 'false');
    preview.hidden = true;
    status('preview-status', '');
    result.hidden = false;
    status('status', 'Your Discord link is ready.');
  } catch (error) {
    if (current !== version) return;
    input.setAttribute('aria-invalid', 'true');
    status('status', error.message, true);
  } finally {
    if (current === version) submit.disabled = false;
  }
});

copy.addEventListener('click', async () => {
  const current = version;
  try {
    await navigator.clipboard.writeText(share.value);
    if (current !== version) return;
    copy.textContent = 'Copied!';
    status('status', 'Link copied. Paste it into Discord.');
  } catch {
    if (current !== version) return;
    share.focus();
    share.select();
    status('status', 'Select Copy from your device to copy the highlighted link.');
  }
});

check.addEventListener('click', async () => {
  if (!preview.hidden) {
    preview.hidden = true;
    check.setAttribute('aria-expanded', 'false');
    check.textContent = 'Preview media';
    return;
  }

  const current = version;
  preview.hidden = false;
  check.setAttribute('aria-expanded', 'true');
  check.textContent = 'Hide preview';
  if (el('media').childElementCount) return;

  check.disabled = true;
  status('preview-status', 'Loading media…');
  try {
    const data = await get('/api/resolve', input.value.trim());
    if (current !== version) return;
    for (const [index, item] of data.media.entries()) {
      const figure = document.createElement('figure');
      const media = document.createElement(item.kind === 'video' ? 'video' : 'img');
      if (item.kind === 'video') {
        media.controls = true;
        media.playsInline = true;
        media.preload = 'none';
        if (item.poster) media.poster = item.poster;
      } else {
        media.alt = 'Slide ' + (index + 1);
        media.loading = 'lazy';
      }
      media.addEventListener('error', () => {
        if (current === version) status('preview-status', 'Some media could not load.', true);
      });
      media.src = item.url;
      figure.append(media);
      el('media').append(figure);
    }

    const { pages, end, total } = data.gallery;
    if (pages > 1) {
      status('preview-status', 'This link previews slides 1–' + end + ' of ' + total + '. Open a set below for its share link.');
      for (let page = 2; page <= pages; page++) {
        const link = document.createElement('a');
        link.className = 'button secondary';
        link.textContent = 'Preview set ' + page;
        link.href = data.share_url + '?page=' + page;
        el('pages').append(link);
      }
    } else {
      status('preview-status', '');
    }
  } catch (error) {
    if (current === version) status('preview-status', error.message, true);
  } finally {
    if (current === version) check.disabled = false;
  }
});
