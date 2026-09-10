import { defineConfig } from "vitepress";

export default defineConfig({
  lang: "en-US",
  title: "Dopsy",
  description: "Read-only, evidence-linked Docker diagnostics.",
  base: "/dopsy/",
  cleanUrls: true,
  lastUpdated: true,
  sitemap: {
    hostname: "https://smn-pascal.github.io/dopsy/",
  },
  head: [
    [
      "link",
      { rel: "icon", href: "/dopsy/favicon.svg", type: "image/svg+xml" },
    ],
    [
      "meta",
      {
        name: "theme-color",
        content: "#ffffff",
        media: "(prefers-color-scheme: light)",
      },
    ],
    [
      "meta",
      {
        name: "theme-color",
        content: "#141816",
        media: "(prefers-color-scheme: dark)",
      },
    ],
    ["meta", { name: "color-scheme", content: "dark light" }],
    ["meta", { property: "og:type", content: "website" }],
    ["meta", { property: "og:site_name", content: "Dopsy" }],
    [
      "meta",
      { property: "og:url", content: "https://smn-pascal.github.io/dopsy/" },
    ],
    [
      "meta",
      { property: "og:title", content: "Dopsy — Read-only Docker diagnostics" },
    ],
    [
      "meta",
      {
        property: "og:description",
        content:
          "Investigate Docker problems through bounded, read-only evidence.",
      },
    ],
    [
      "meta",
      {
        property: "og:image",
        content:
          "https://smn-pascal.github.io/dopsy/brand/dopsy-social-preview.png",
      },
    ],
    ["meta", { name: "twitter:card", content: "summary_large_image" }],
    [
      "meta",
      {
        name: "twitter:title",
        content: "Dopsy — Read-only Docker diagnostics",
      },
    ],
    [
      "meta",
      {
        name: "twitter:description",
        content:
          "Investigate Docker problems through bounded, read-only evidence.",
      },
    ],
    [
      "meta",
      {
        name: "twitter:image",
        content:
          "https://smn-pascal.github.io/dopsy/brand/dopsy-social-preview.png",
      },
    ],
  ],
  themeConfig: {
    logo: {
      src: "/brand/dopsy-mark.svg",
      alt: "Dopsy",
      width: 28,
      height: 28,
    },
    nav: [
      {
        text: "Guide",
        items: [
          { text: "Getting started", link: "/guide/getting-started" },
          { text: "How it works", link: "/guide/how-it-works" },
        ],
      },
      { text: "Configuration", link: "/configuration/ai-provider" },
      { text: "Security", link: "/security/read-only" },
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
    search: {
      provider: "local",
      options: {
        translations: {
          button: {
            buttonText: "Search docs",
            buttonAriaLabel: "Search Dopsy documentation",
          },
        },
      },
    },
    outline: {
      level: [2, 3],
      label: "On this page",
    },
    editLink: {
      pattern: "https://github.com/smn-pascal/dopsy/edit/main/docs/:path",
      text: "Edit this page on GitHub",
    },
    lastUpdated: {
      text: "Updated",
    },
    docFooter: {
      prev: "Previous",
      next: "Next",
    },
    footer: {
      message: "Dopsy is an early development preview. Keep it local.",
      copyright: "Read-only Docker diagnostics.",
    },
  },
});
