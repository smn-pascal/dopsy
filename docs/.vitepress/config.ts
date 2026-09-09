import { defineConfig } from "vitepress";

export default defineConfig({
  title: "Dopsy",
  description: "Understand your containers.",
  cleanUrls: true,
  head: [["meta", { name: "theme-color", content: "#0d1513" }]],
  themeConfig: {
    nav: [
      { text: "Guide", link: "/guide/getting-started" },
      { text: "Security", link: "/security/read-only" },
      { text: "GitHub", link: "https://github.com/smn-pascal/dopsy" },
    ],
    sidebar: [
      {
        text: "Guide",
        items: [
          { text: "What is Dopsy?", link: "/" },
          { text: "Getting started", link: "/guide/getting-started" },
          { text: "How it works", link: "/guide/how-it-works" },
        ],
      },
      {
        text: "Configuration",
        items: [{ text: "AI providers", link: "/configuration/ai-provider" }],
      },
      {
        text: "Security",
        items: [{ text: "Read-only design", link: "/security/read-only" }],
      },
    ],
    socialLinks: [
      { icon: "github", link: "https://github.com/smn-pascal/dopsy" },
    ],
    footer: {
      message:
        "Self-hosted container diagnostics. Your infrastructure, your model.",
    },
  },
});
