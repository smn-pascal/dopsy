---
layout: home

hero:
  name: Dopsy
  text: Understand your containers.
  tagline: Ask what happened. Get an evidence-based diagnosis without giving an AI permission to change your system.
  actions:
    - theme: brand
      text: Get started
      link: /guide/getting-started
    - theme: alt
      text: View on GitHub
      link: https://github.com/smn-pascal/dopsy

features:
  - title: Ask, don't search
    details: Describe a problem in plain language. Dopsy gathers only the container data needed for that question.
  - title: Read-only by design
    details: The diagnostic agent receives no start, stop, restart, exec, delete, or deployment tools.
  - title: Bring your own model
    details: Connect an OpenAI-compatible cloud, company, or local endpoint. Dopsy does not operate a central AI service.
---

## The first milestone

Dopsy currently focuses on one complete path: list containers, inspect their state, retrieve bounded logs and statistics, and explain the evidence in a diagnostic chat.

## Evidence, not guesses

![Dopsy showing an evidence-based out-of-memory diagnosis](/images/dopsy-diagnosis.png)

This example runs entirely against Dopsy's deterministic local demo data. The same workspace connects each real diagnosis to the container facts and read-only tools that support it.
