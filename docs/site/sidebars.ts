import type { SidebarsConfig } from "@docusaurus/plugin-content-docs";

const sidebars: SidebarsConfig = {
  docsSidebar: [
    {
      type: "category",
      label: "Getting Started",
      items: ["intro", "quickstart", "architecture"],
    },
    {
      type: "category",
      label: "Protocol Specification",
      items: [
        "protocol/identity",
        "protocol/discovery",
        "protocol/onboarding",
        "protocol/commerce",
        "protocol/wire-formats",
        "protocol/error-codes",
      ],
    },
    {
      type: "category",
      label: "Server",
      items: [
        "server/installation",
        "server/configuration",
        "server/deployment",
      ],
    },
    {
      type: "category",
      label: "SDKs",
      items: ["sdks/typescript", "sdks/python"],
    },
    {
      type: "category",
      label: "Operator",
      items: [
        "operator/installation",
        "operator/crds",
        "operator/deployment",
      ],
    },
    {
      type: "link",
      label: "API Reference",
      href: "/api",
    },
  ],
};

export default sidebars;
