// 导出页面上可见元素的盒子与关键计算样式（cdp.mjs --eval 使用），作为一比一还原的尺寸依据。
// 只导出某个子树：先 --js 'window.__dumpRoot = "选择器"'；条数上限 window.__dumpLimit（默认 400）。
// 每行：标签.类名(截断) | x,y,w,h | 字体 | 颜色/背景/边框/圆角/内边距
(() => {
  const limit = Number(window.__dumpLimit || 400);
  const skipInsideCardsAfter = 1; // 只展开第一张卡片
  const out = [];
  const seenCards = new Set();
  const keep = new Set(["DIV", "ASIDE", "NAV", "MAIN", "BUTTON", "A", "INPUT", "H1", "H2", "H3", "P", "SPAN", "IMG", "UL", "LI", "SVG"]);
  const vw = innerWidth;
  const vh = innerHeight;
  const tidy = c => (c && c !== "rgba(0, 0, 0, 0)" && c !== "none" && c !== "0px" && c !== "normal") ? c : "";
  const root = window.__dumpRoot ? document.querySelector(window.__dumpRoot) : document.body;
  for (const el of root.querySelectorAll("*")) {
    if (out.length >= limit)
      break;
    if (!keep.has(el.tagName.toUpperCase()))
      continue;
    const card = el.closest(".group.relative.flex.w-full.flex-col");
    if (card) {
      if (!seenCards.has(card) && seenCards.size >= skipInsideCardsAfter && card !== el)
        continue;
      seenCards.add(card);
    }
    const r = el.getBoundingClientRect();
    if (r.width === 0 || r.height === 0 || r.bottom < 0 || r.top > vh || r.right < 0 || r.left > vw)
      continue;
    const s = getComputedStyle(el);
    if (s.visibility === "hidden" || s.display === "none")
      continue;
    const cls = (el.getAttribute("class") || "").replace(/\s+/g, " ").slice(0, 90);
    const text = el.childNodes.length && [...el.childNodes].some(n => n.nodeType === 3 && n.textContent.trim())
      ? ` "${el.textContent.trim().slice(0, 24)}"`
      : "";
    const parts = [
      `${el.tagName.toLowerCase()}${cls ? `.${cls}` : ""}${text}`,
      `${r.x.toFixed(1)},${r.y.toFixed(1)},${r.width.toFixed(1)}x${r.height.toFixed(1)}`,
    ];
    if (text || el.tagName === "INPUT")
      parts.push(`font ${s.fontSize}/${s.lineHeight} w${s.fontWeight} ${s.fontFamily.slice(0, 40)} color ${s.color}`);
    const box = [
      tidy(s.backgroundColor) && `bg ${s.backgroundColor}`,
      tidy(s.backgroundImage) && `bgimg ${s.backgroundImage.slice(0, 80)}`,
      s.borderTopWidth !== "0px" || s.borderRightWidth !== "0px" || s.borderBottomWidth !== "0px" || s.borderLeftWidth !== "0px"
        ? `border ${s.borderTopWidth} ${s.borderRightWidth} ${s.borderBottomWidth} ${s.borderLeftWidth} ${s.borderTopColor}`
        : "",
      tidy(s.borderRadius) && `radius ${s.borderRadius}`,
      s.padding !== "0px" ? `pad ${s.padding}` : "",
      tidy(s.boxShadow) && `shadow ${s.boxShadow.slice(0, 80)}`,
      s.opacity !== "1" ? `opacity ${s.opacity}` : "",
      tidy(s.maskImage || s.webkitMaskImage) ? `icon ${el.className.match?.(/i-mdi-[\w-]+/)?.[0] ?? ""} color ${s.backgroundColor || s.color}` : "",
    ].filter(Boolean);
    if (box.length)
      parts.push(box.join("; "));
    out.push(parts.join(" | "));
  }
  return out.join("\n");
})();
