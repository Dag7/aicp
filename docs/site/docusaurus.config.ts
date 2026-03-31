import { themes as prismThemes } from "prism-react-renderer";
import type { Config } from "@docusaurus/types";
import type * as Preset from "@docusaurus/preset-classic";

const config: Config = {
  title: "AICP Documentation",
  tagline: "Agentic Identity and Commerce Protocol",
  favicon: "img/favicon.ico",

  url: "https://docs.aicp.dev",
  baseUrl: "/",

  organizationName: "aicp-protocol",
  projectName: "aicp",

  onBrokenLinks: "throw",
  onBrokenMarkdownLinks: "warn",

  i18n: {
    defaultLocale: "en",
    locales: ["en"],
  },

  presets: [
    [
      "classic",
      {
        docs: {
          sidebarPath: "./sidebars.ts",
          editUrl: "https://github.com/aicp-protocol/aicp/tree/main/docs/site/",
        },
        theme: {
          customCss: "./src/css/custom.css",
        },
      } satisfies Preset.Options,
    ],
  ],

  themeConfig: {
    colorMode: {
      defaultMode: "dark",
      disableSwitch: false,
      respectPrefersColorScheme: true,
    },
    navbar: {
      title: "AICP",
      items: [
        {
          type: "docSidebar",
          sidebarId: "docsSidebar",
          position: "left",
          label: "Docs",
        },
        {
          href: "/api",
          label: "API",
          position: "left",
        },
        {
          href: "https://github.com/aicp-protocol/aicp/tree/main/spec",
          label: "Protocol Spec",
          position: "left",
        },
        {
          href: "https://github.com/aicp-protocol/aicp",
          label: "GitHub",
          position: "right",
        },
      ],
    },
    footer: {
      style: "dark",
      links: [
        {
          title: "Docs",
          items: [
            { label: "Getting Started", to: "/docs/intro" },
            { label: "Quickstart", to: "/docs/quickstart" },
          ],
        },
        {
          title: "Community",
          items: [
            {
              label: "GitHub Discussions",
              href: "https://github.com/aicp-protocol/aicp/discussions",
            },
          ],
        },
        {
          title: "More",
          items: [
            {
              label: "GitHub",
              href: "https://github.com/aicp-protocol/aicp",
            },
            {
              label: "Protocol Spec",
              href: "https://github.com/aicp-protocol/aicp/tree/main/spec",
            },
          ],
        },
      ],
      copyright: `Built by the AICP community. Licensed under Apache 2.0.`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
      additionalLanguages: ["go", "typescript", "python", "json", "yaml", "bash"],
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
