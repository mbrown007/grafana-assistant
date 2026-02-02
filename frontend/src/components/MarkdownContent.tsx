import React from 'react';

interface MarkdownContentProps {
  content: string;
  className?: string;
}

export function MarkdownContent({ content, className = '' }: MarkdownContentProps) {
  const normalizeContent = (text: string): string => {
    let normalized = text.replace(/\r\n/g, '\n');
    normalized = normalized.replace(/([^\n])\s+(\d+\.\s)/g, '$1\n$2');
    normalized = normalized.replace(/([^\n])\s+([-*+]\s)/g, '$1\n$2');
    return normalized;
  };

  const isSafeUrl = (url: string): boolean => {
    try {
      const parsed = new URL(url, window.location.href);
      return parsed.protocol === 'http:' || parsed.protocol === 'https:';
    } catch {
      return false;
    }
  };

  const sanitizeLanguage = (lang: string): string => {
    return lang.replace(/[^a-zA-Z0-9-]/g, '') || 'plaintext';
  };

  const parseInlineMarkdown = (text: string): Array<React.ReactNode> => {
    const elements: Array<React.ReactNode> = [];
    let lastIndex = 0;
    let key = 0;
    const regex = /\*\*(.+?)\*\*|__(.+?)__|\*(.+?)\*|_(.+?)_|`([^`]+)`|\[([^\]]+)\]\(([^)]+)\)/g;
    let match: RegExpExecArray | null = null;

    while ((match = regex.exec(text)) !== null) {
      if (match.index > lastIndex) {
        elements.push(text.slice(lastIndex, match.index));
      }
      if (match[1] || match[2]) {
        elements.push(<strong key={`bold-${key++}`}>{match[1] || match[2]}</strong>);
      } else if (match[3] || match[4]) {
        elements.push(<em key={`italic-${key++}`}>{match[3] || match[4]}</em>);
      } else if (match[5]) {
        elements.push(
          <code key={`code-${key++}`} className="bg-muted px-1.5 py-0.5 rounded text-sm break-all">
            {match[5]}
          </code>
        );
      } else if (match[6] && match[7]) {
        if (isSafeUrl(match[7])) {
          elements.push(
            <a key={`link-${key++}`} href={match[7]} target="_blank" rel="noreferrer">
              {match[6]}
            </a>
          );
        } else {
          elements.push(<span key={`link-${key++}`}>{match[6]}</span>);
        }
      }
      lastIndex = regex.lastIndex;
    }

    if (lastIndex < text.length) {
      elements.push(text.slice(lastIndex));
    }

    return elements;
  };

  const parseMarkdown = (text: string): Array<React.ReactNode> => {
    const elements: Array<React.ReactNode> = [];
    let key = 0;
    const normalized = normalizeContent(text);
    const lines = normalized.split('\n');
    let i = 0;

    while (i < lines.length) {
      const line = lines[i];
      if (!line.trim()) {
        i += 1;
        continue;
      }

      if (line.trim().startsWith('```')) {
        const language = sanitizeLanguage(line.trim().slice(3));
        const codeLines: string[] = [];
        i += 1;
        while (i < lines.length && !lines[i].trim().startsWith('```')) {
          codeLines.push(lines[i]);
          i += 1;
        }
        i += 1;
        elements.push(
          <pre key={`code-${key++}`} className="bg-muted rounded-lg p-4 overflow-x-auto max-w-full">
            <code className={`language-${language}`}>{codeLines.join('\n')}</code>
          </pre>
        );
        continue;
      }

      const headingMatch = line.match(/^(#{1,6})\s+(.*?)$/);
      if (headingMatch) {
        const level = headingMatch[1].length;
        const headingText = headingMatch[2];
        const HeadingTag = `h${level}` as keyof JSX.IntrinsicElements;
        elements.push(
          React.createElement(
            HeadingTag,
            {
              key: `heading-${key++}`,
              className: `mt-5 mb-2 font-semibold ${level === 1 ? 'text-xl' : level === 2 ? 'text-lg' : 'text-base'}`,
            },
            parseInlineMarkdown(headingText)
          )
        );
        i += 1;
        continue;
      }

      if (line.trim().match(/^\d+\.\s/)) {
        const listItems: Array<{ main: string; subItems: Array<string> }> = [];
        while (i < lines.length) {
          const currentLine = lines[i];
          const numberedMatch = currentLine.match(/^\d+\.\s+(.*)/);
          if (numberedMatch) {
            listItems.push({ main: numberedMatch[1], subItems: [] });
            i += 1;
            continue;
          }
          const subItemMatch = currentLine.match(/^\s+[-*]\s+(.*)/);
          if (subItemMatch && listItems.length > 0) {
            listItems[listItems.length - 1].subItems.push(subItemMatch[1]);
            i += 1;
            continue;
          }
          if (!currentLine.trim()) {
            let nextNonEmpty = i + 1;
            while (nextNonEmpty < lines.length && !lines[nextNonEmpty].trim()) {
              nextNonEmpty += 1;
            }
            if (nextNonEmpty < lines.length && lines[nextNonEmpty].match(/^\d+\.\s/)) {
              i += 1;
              continue;
            }
            break;
          }
          break;
        }

        elements.push(
          <ol key={`ol-${key++}`} className="my-2 ml-5">
            {listItems.map((item, idx) => (
              <li key={`li-${key++}-${idx}`}>
                {parseInlineMarkdown(item.main)}
                {item.subItems.length > 0 && (
                  <ul className="mt-1 ml-4">
                    {item.subItems.map((sub, subIdx) => (
                      <li key={`sub-${key++}-${subIdx}`}>{parseInlineMarkdown(sub)}</li>
                    ))}
                  </ul>
                )}
              </li>
            ))}
          </ol>
        );
        continue;
      }

      if (line.trim().match(/^[-*+]\s/)) {
        const items: string[] = [];
        while (i < lines.length && lines[i].trim().match(/^[-*+]\s/)) {
          items.push(lines[i].replace(/^[-*+]\s/, ''));
          i += 1;
        }
        elements.push(
          <ul key={`ul-${key++}`} className="my-2 ml-5">
            {items.map((item, idx) => (
              <li key={`bullet-${key++}-${idx}`}>{parseInlineMarkdown(item)}</li>
            ))}
          </ul>
        );
        continue;
      }

      const paragraphLines: string[] = [];
      while (
        i < lines.length &&
        lines[i].trim() &&
        !lines[i].match(/^#{1,6}\s/) &&
        !lines[i].match(/^\d+\.\s/) &&
        !lines[i].match(/^[-*+]\s/) &&
        !lines[i].startsWith('```')
      ) {
        paragraphLines.push(lines[i]);
        i += 1;
      }

      if (paragraphLines.length > 0) {
        elements.push(
          <p key={`paragraph-${key++}`} className="my-2">
            {parseInlineMarkdown(paragraphLines.join('\n'))}
          </p>
        );
      }
    }

    return elements;
  };

  return <div className={`markdown ${className}`}>{parseMarkdown(content)}</div>;
}
