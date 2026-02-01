import { render, screen } from '@testing-library/react';
import { MarkdownContent } from '../MarkdownContent';

describe('MarkdownContent', () => {
  it('renders plain text', () => {
    render(<MarkdownContent content="Hello world" />);
    expect(screen.getByText('Hello world')).toBeInTheDocument();
  });

  it('renders bold text', () => {
    render(<MarkdownContent content="This is **bold** text" />);
    expect(screen.getByText('bold').tagName).toBe('STRONG');
  });

  it('renders italic text', () => {
    render(<MarkdownContent content="This is *italic* text" />);
    expect(screen.getByText('italic').tagName).toBe('EM');
  });

  it('renders inline code', () => {
    render(<MarkdownContent content="Use `console.log` here" />);
    const code = screen.getByText('console.log');
    expect(code.tagName).toBe('CODE');
    expect(code).toHaveClass('inline-code');
  });

  it('renders code blocks', () => {
    const content = '```javascript\nconst x = 1;\n```';
    render(<MarkdownContent content={content} />);
    expect(screen.getByText('const x = 1;')).toBeInTheDocument();
  });

  it('renders unordered lists', () => {
    const content = '- Item one\n- Item two\n- Item three';
    render(<MarkdownContent content={content} />);
    expect(screen.getByText('Item one')).toBeInTheDocument();
    expect(screen.getByText('Item two')).toBeInTheDocument();
    expect(screen.getByText('Item three')).toBeInTheDocument();
  });

  it('renders ordered lists', () => {
    const content = '1. First\n2. Second\n3. Third';
    render(<MarkdownContent content={content} />);
    expect(screen.getByText('First')).toBeInTheDocument();
    expect(screen.getByText('Second')).toBeInTheDocument();
    expect(screen.getByText('Third')).toBeInTheDocument();
  });

  it('renders links with safe URLs', () => {
    render(<MarkdownContent content="Visit [Example](https://example.com)" />);
    const link = screen.getByText('Example');
    expect(link.tagName).toBe('A');
    expect(link).toHaveAttribute('href', 'https://example.com');
    expect(link).toHaveAttribute('target', '_blank');
  });

  it('does not render links with unsafe URLs', () => {
    render(<MarkdownContent content="Click [here](javascript:alert(1))" />);
    const span = screen.getByText('here');
    expect(span.tagName).toBe('SPAN');
  });

  it('renders headings', () => {
    render(<MarkdownContent content="## My Heading" />);
    const heading = screen.getByText('My Heading');
    expect(heading.tagName).toBe('H2');
  });

  it('applies custom className', () => {
    const { container } = render(<MarkdownContent content="test" className="custom" />);
    expect(container.firstChild).toHaveClass('markdown', 'custom');
  });
});
