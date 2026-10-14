---
title: "We Treat Applications as Products. Why Is the Platform Still a Black Box? Introducing Platform BOM (PBOM)"
published: true
description: "Platform teams build the foundations every product team depends on. A practical case for making the platform visible, versioned and understandable, with an open-source Platform BOM."
tags: platformengineering, kubernetes, devops, opensource
layout: article
permalink: /platform-as-a-product-platform-bom/
date: 2026-10-14
---

An application team can point to the feature it shipped this week. A platform team may have spent that same week making deployments safer, improving cluster reliability, tightening identity, or removing a recurring obstacle from dozens of teams.

The work matters. Its impact is just harder to see.

As a platform engineer, I have sometimes felt that platform work is not seen with the same importance as application development. I do not think this is because application teams misunderstand infrastructure, or because every organization undervalues its platform. It is often a visibility problem: application features connect directly to customers, while platform improvements create value indirectly, across many teams and over time.

That difference can make the platform look like a cost center or a collection of infrastructure tickets, even when it is a product that application teams rely on every day.

I want to help change that perspective. Platform engineering deserves to be treated as software product engineering: with users, capabilities, releases, feedback, and a clear account of what is running. That is the idea behind [Platform BOM](https://github.com/ravibagri5/platform-bom).

## A platform is more than its infrastructure

An internal platform is not just Kubernetes, a cloud account, or a deployment pipeline. It is the set of capabilities and experiences that help teams build, deliver, and operate their products.

That may include Kubernetes, GitOps, networking, identity, policy, observability, data services, and the self-service workflows that connect them. The implementation can change while the capability a team depends on stays the same. A platform might replace one database provider with another without changing the promise that teams can request a supported PostgreSQL service.

This is why “platform as a product” is more than a slogan. A product has users and a purpose. It has capabilities, a roadmap, support expectations, and a lifecycle. It should evolve in response to user needs, not just accumulate tools because they are available.

The [CNCF Platforms White Paper](https://tag-app-delivery.cncf.io/whitepapers/platforms/) describes platforms as curated capabilities presented around the needs of internal users. It also calls out user experience, documentation, self-service, and reducing cognitive load as important platform attributes. [DORA’s platform engineering guidance](https://dora.dev/capabilities/platform-engineering/) similarly emphasizes product thinking, developer needs, and measuring outcomes.

So the industry is not ignoring platform engineering. The discipline is increasingly recognized. But recognition does not automatically make a platform’s scope, health, or impact visible inside every organization. That is the gap I want to work on.

## The questions we should be able to answer

If the platform is a product, we should be able to describe it without reconstructing it from memory every time someone asks:

- What capabilities does the platform offer to application teams?
- Which components implement those capabilities?
- What is the declared platform release?
- Which versions are actually running in development, staging, and production?
- Where have environments drifted from their intended release?
- Which upstream updates deserve attention?

In many organizations, the answers are spread across Helm releases, Git repositories, cluster APIs, dashboards, spreadsheets, and the knowledge of a few engineers. Each tool can be useful and still leave the platform itself difficult to describe as one product.

An application catalog can tell us which services exist and who owns them. A Kubernetes dashboard can show resources in a cluster. An SBOM describes the packages in a software artifact. These tools answer different questions. They do not, by themselves, connect a platform’s user-facing capabilities to the components and versions behind them across environments.

## An inventory for the platform itself

Software teams use a Software Bill of Materials to describe what an artifact contains. Platform BOM applies a similar idea at a different level: describe a platform’s components, capabilities, releases, and environments, then compare what was intended with evidence of what is running.

To be precise, **Platform Bill of Materials (PBOM) is a concept proposed by this project, not an established industry standard**. The project does not claim CNCF endorsement or industry-wide adoption. The goal is to explore a useful model and make it practical enough for platform teams to try.

[Platform BOM](https://github.com/ravibagri5/platform-bom) is an open-source, read-only inventory for platform teams. It connects a Git-declared platform and its releases with evidence discovered from Kubernetes, Helm, and Crossplane. It can compare environments with their target releases and show upstream versions that may need attention.

![Platform BOM overview showing platform offerings, environment alignment, and upstream health](https://raw.githubusercontent.com/ravibagri5/platform-bom/main/docs/screenshots/platform-compact.jpg)

*A compact view of the platform inventory. The screenshot is shown in the article body at its natural aspect ratio, rather than cropped as a cover image.*

The model separates a few ideas that often get mixed together:

- **Offerings** describe what platform users receive, such as GitOps delivery, metrics, or managed PostgreSQL.
- **Components** describe the technologies behind those offerings.
- **Platform releases** describe versioned baselines.
- **Environments and inventories** describe where the platform runs and what discovery observed there.

That separation matters. Users depend on capabilities; platform teams operate implementations. If an implementation changes, the platform can record the change without pretending the user-facing capability disappeared.

## What this looks like in practice

Platform BOM is deliberately observational. It does not install or upgrade components, and it does not compete with GitOps. Your existing deployment tools remain responsible for making changes; PBOM helps describe and compare the result.

For example, a platform team can declare that development, staging, and production should run a particular platform release. Discovery then gives the team evidence about the components found in each environment. If production is still on an earlier release, the difference is visible instead of being hidden in a spreadsheet or rediscovered during an incident.

The project includes a Helm chart and a getting-started guide. To try the released `v0.9.0` chart on a cluster you are authorized to inspect:

```sh
helm upgrade --install pbom oci://ghcr.io/ravibagri5/charts/platform-bom \
  --version 0.9.0 \
  --namespace platform-bom --create-namespace --wait

kubectl -n platform-bom port-forward svc/pbom-platform-bom 8080:80
```

Then open <http://127.0.0.1:8080>. The default service account is read-only. Before sharing the UI, configure sign-in or an authenticating proxy, as described in the [authentication guide](https://github.com/ravibagri5/platform-bom/blob/main/docs/authentication.md). The [getting-started guide](https://github.com/ravibagri5/platform-bom/blob/main/docs/getting-started.md) walks through describing a platform and its release baselines in YAML.

The project is at `v0.9.0` today. It includes multi-environment inventory, component and offering views, release comparison, upstream version information, cloud workload identity for supported managed Kubernetes environments, and OIDC or proxy-based authentication. Those are current capabilities, not a claim that every platform concern is solved.

## What a BOM can and cannot prove

A platform inventory can make composition, declared releases, observed versions, and drift easier to discuss. That can support upgrade planning, operational reviews, and conversations about which capabilities the platform provides.

It cannot prove that developers are satisfied. It cannot show on its own that onboarding is faster, cognitive load is lower, or product outcomes have improved. Those questions need user feedback and outcome measures: task success, onboarding time, satisfaction, delivery performance, and other signals that make sense for the organization.

That distinction is important to me. A BOM should not become another dashboard used to justify building a platform nobody asked for. It should make the platform easier to understand, then help platform teams have better conversations with the people who use it.

## Where the project is heading

The roadmap is a statement of direction, not a promise of dates. Areas under consideration and active development include:

- A versioned API and integrations with tools such as Backstage and MCP clients.
- More evidence from GitOps controllers and infrastructure-as-code systems.
- Better upstream intelligence, including support windows, security advisories, and compatibility information.
- Interoperability work, including exporting a platform release in an established format such as CycloneDX.
- A shared PBOM specification, if the model proves useful beyond this project.

These are ways to make the platform easier to inspect and integrate with existing workflows. They are not reasons to replace the systems that already deploy, secure, or observe it.

## Make the platform legible

Platform engineering is software engineering applied to shared capabilities and developer experience. If we expect platform teams to own products, we should also make it possible to describe those products: what they offer, how they are assembled, which release is intended, and what is actually running.

That is the problem I am exploring with Platform BOM. It is early, the resource model is still evolving, and the PBOM concept is not a standard. I would rather be explicit about those limits than imply the problem is already solved.

If you work on an internal platform, I would be interested in how your team answers these questions today. What do you track? What remains tribal knowledge? Which evidence sources or integrations would make a platform inventory genuinely useful in your workflow?

Explore the project, try the [getting-started guide](https://github.com/ravibagri5/platform-bom/blob/main/docs/getting-started.md), or share feedback through [GitHub Discussions](https://github.com/ravibagri5/platform-bom/discussions).
