import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";
import rehypeKatex from "rehype-katex";
import "katex/dist/katex.min.css";

function normalizeLatex(text: string): string {
  return text
    .replace(/\\\[([\s\S]+?)\\\]/g, (_m, inner: string) => `$$${inner}$$`)
    .replace(/\\\((.+?)\\\)/g, (_m, inner: string) => `$$${inner}$$`);
}

export function Markdown({ content }: { content: string }) {
  return (
    <div className="markdown-body">
      <ReactMarkdown
        remarkPlugins={[remarkGfm, [remarkMath, { singleDollarTextMath: false }]]}
        rehypePlugins={[rehypeKatex]}
      >
        {normalizeLatex(content)}
      </ReactMarkdown>
    </div>
  );
}
