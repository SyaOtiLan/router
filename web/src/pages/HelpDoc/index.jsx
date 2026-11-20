import React, { useMemo, useRef } from 'react';
import { marked } from 'marked';
import { useTranslation } from 'react-i18next';
import clientGuideMarkdown from '../../../../docs/客户端/README.md?raw';
import { AppFilterHeader } from '../../router-ui';
import useCopyableCodeBlocks from './useCopyableCodeBlocks';
import './HelpDoc.css';

const API_BASE = typeof window !== 'undefined' ? window.location.origin : '';
const clientGuideImages = import.meta.glob('../../../../docs/客户端/images/*', {
  eager: true,
  query: '?url',
  import: 'default',
});

function buildClientGuide(markdown) {
  const body = markdown.replace(/^# .+\r?\n+/, '');
  const renderer = new marked.Renderer();
  renderer.heading = (text, level, raw, slugger) => {
    const id = slugger.slug(raw);
    return `<h${level} id="${id}">${text}</h${level}>`;
  };
  renderer.image = (href, imageTitle, text) => {
    const relativePath = decodeURIComponent(href).replace(/^\.\//, '');
    const imageUrl = clientGuideImages[`../../../../docs/客户端/${relativePath}`];
    if (!imageUrl) return false;
    const titleAttribute = imageTitle ? ` title="${imageTitle}"` : '';
    return `<img src="${imageUrl}" alt="${text}"${titleAttribute}>`;
  };

  const toc = [];
  const slugger = new marked.Slugger();
  marked.lexer(body).forEach((token) => {
    if (token.type !== 'heading') return;
    const id = slugger.slug(token.text);
    if (token.depth > 1 && token.depth <= 3) {
      toc.push({
        level: token.depth,
        id,
        text: marked.parseInline(token.text),
      });
    }
  });

  const html = marked.parse(body, { renderer }).replace(
    /(<pre><code\b[^>]*>[\s\S]*?<\/code><\/pre>)/g,
    (block) =>
      /YOUR_YEYING_ROUTER_API_KEY|YOUR_OPENAI_API_KEY/.test(block)
        ? `${block}<div class="router-help-doc-token-tip">需要令牌？<a href="/workspace/token">获取</a></div>`
        : block,
  );
  const nav = toc
    .map(({ level, id, text }) => {
      const sub = level > 2 ? ' toc-item--sub' : '';
      return `<a href="#${id}" class="toc-item${sub}">${text}</a>`;
    })
    .join('');

  return `<div class="usage-doc-page"><div class="doc-body"><aside class="doc-sidebar"><div class="toc-header">目录</div><div class="toc-divider"></div><nav class="toc-items">${nav}</nav></aside><main class="doc-content"><article class="doc-section markdown-section">${html}</article></main></div></div>`
    .replaceAll('https://router.yeying.pub', API_BASE);
}

const HelpDoc = () => {
  const { t } = useTranslation();
  const containerRef = useRef(null);
  const html = useMemo(() => buildClientGuide(clientGuideMarkdown), []);

  useCopyableCodeBlocks(containerRef, [html]);

  return (
    <div className='dashboard-container'>
      <AppFilterHeader
        breadcrumbs={[
          { key: 'workspace', label: t('header.user_workspace') },
          { key: 'help', label: t('header.help') },
          { key: 'cli-guide', label: t('header.cli_guide'), active: true },
        ]}
        title={t('header.cli_guide')}
      />
      <div
        ref={containerRef}
        className='router-help-doc-page'
        dangerouslySetInnerHTML={{ __html: html }}
      />
    </div>
  );
};

export default HelpDoc;
