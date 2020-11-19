# Designdocs

## Goal

Articulate the reasons why we write designdocs and propose that the definition of a plan is that it is written down.

## Problem Statement

If we don't capture, on paper, in the open, the design intent behind important parts of Filecoin we experience at least the following negative consequences:

  - **Wasted time**: when we don't take the time to formalize designs we often end up building the wrong thing or supporting a bunch of stuff that we don't want. We also waste time at a lower level, working within systems that are painful to use because we didn't take the time to fully think through how they should work up-front.
  - **Missed opportunities to engage the community**: when we don't write designs we miss benefitting from the many points of view and specialized knowledge in the community. We also miss the opportunity to come together *as* a community. Engaging on design proposals brings us together. It’s exciting and energizing.
  - **Inability for the community to engage.** This is less obvious flip side of the above. Designs have to exist on paper so that they are accessible to current and future collaborators. It can be frustrating to build, change, or work with something you don't understand the design of. You spend time trying to reverse engineer design intent instead of doing the thing you want to. The worst part is that unless there is a design on paper then nobody can run with or extend it.
  
## Proposal

1. Write designdocs for important components and features, not just ahead of time but in circumstances where there is uncertainty or confusion.
1. Adopt the attitude that there is no plan (design) unless it is written down and circulated in the open. The norm is: __show me the plan or it doesn’t exist__.

## What is a designdoc?

A designdoc is a short document clearly articulating a problem, a solution to that problem, and why we think the proposal is the right solution. It should include at least:
- **Problem Statement**: what problem are we solving and why?
- **Requirements and anti-requirements**: enumerate the assumptions and constraints and as importantly the non-goals and anti-requirements.
- **Proposal & Rationale**: a sketch of the thing and most importantly why this thing and not something else. What do we gain if we do this, what do we lose if we do not?
- **Alternatives**: what are we not doing and why not?

An example of a designdoc is this document, for a part of our development process.

The most important thing a designdoc does is capture *design intent*. It shouldn't just state the solution to the problem, it should present a solution along with the reasoning that went into why that solution and not something else. It should capture the constraints, considerations, and implications of the solution. 

It is important to understand that the goal of a design doc is not to defend a position, but to discover truths. The reason to take a position isn't to be right, it's to create a hypothesis that we can collectively evaluate together.

## Benefits

Capturing design intent on paper, in the open, in a designdoc:
- **Subjects it to thoughtful criticism**.
- **Clearly articulates what is and is not important**.
- **Synchronizes the community on important questions**. 
- **Disseminates information in a scalable fashion**. 

## What is a designdoc not?

A designdoc is not a spec: you use a designdoc to rationalize a plan ahead of implementation and it should have less detail and more discussion than a spec, and it should capture far more of the *why* than a spec would. A designdoc is not a github issue, though it may be captured in one: a designdoc typically ties together a number of open questions or problems into a coherent whole. 

A designdoc *does not* have to:
- be kept up to date. It's a tool for capturing knowledge, formalizing our thinking, and synchronizing the team. Once it has served those purposes we don't necessarily need to tend to it.
- be long. In fact it should not be long. Several pages, by six or eight it starts to feel bloated.
- solve every problem. It should contain sufficient detail to convince readers that it contains a good idea, but not so much as to obscure the big picture or risk bike shedding on details that can fall out later.

## Alternatives

##### Something like BIP or EIP. 

[Protocol-level changes](https://github.com/filecoin-project/specs/blob/master/process.md) go through a process 
like that. Possibly we should adopt something similar for venus. But for now, we have designdocs.

##### Use specifications.

[Alex Russel sums it up pretty nicely](https://docs.google.com/document/d/1cJs7GkdQolqOHns9k6v1UjCUb_LqTFVjZM-kc3TbNGI/edit). But basically:

Specs do something different than designdocs—they enable interop. Compare the [Service Worker Explainer (designdoc)](https://github.com/w3c/ServiceWorker/blob/master/explainer.md) to the [Service Worker Specification](https://www.w3.org/TR/service-workers-1/). Which one would help you more if you were beginning to contribute code, investigating a security vulnerability and needed to get up to speed, or if you were asked to give feedback on a design?

Specs are a contract between implementations and users. And like all contracts, they are dense and detail-oriented. This makes them a difficult way to understand the big picture of a design. It is tempting to use specs to serve both this contractual use case and also the proposal-evaluating use case we're talking about here. You could think of this doc as a proposal to use designdocs for the proposal-evaluating use case, as they seem to be a more appropriate tool for that task.

Obviously, we need specs too. But they often come later in the lifecycle, and are less useful for articulating system-level design than a designdoc.

##### Github issues.

We could capture a designdoc in a github issue, and this may in fact
be a good convention. But let's not mistake dicussions in an issue for
a clear summary of the conclusion. Sometimes you want the latter without 
having to parse all the discussions that went into it.

## FAQ

##### How do I know what warrants a designdoc?
A designdoc is a tool so like any apply it judiciously. If you’re building something that other devs are going to work within, or with, and it’s going to take you weeks or months to do it, it’s probably a good candidate for a designdoc. Or if it’s something critically important to the project, that’s probably also worth one. Or if it's a separate thingy with its own structure, then too.

##### But what if the designdoc is wrong? 
With probability 1 it will be wrong. The point is to explicitly communicate what we are and are not trying to accomplish and to rationalize a proposal. It’s a planning tool and as soon as it is finished reality will start to diverge. But as they say: plans are worthless but planning is indispensable.

##### What if I’m not sure what the solution to this problem should be? 
That’s OK. We are never sure. Try really hard to take a position. Nobody is going to hold it against you if we all discover together that the position was wrong. The hardest part of design is understanding the problem well enough to propose any solution. It facilitates collaboration and progress to write down a clear description of a problem and one hypothesized best solution.

##### I don’t like / want to write design docs. I want to write code.
OK, but you’re just limiting yourself. Getting good at writing designs enables you to increase your leverage to accomplish bigger, better things. You’re also giving up influence over outcomes if you don’t get good at articulating your ideas. But if you really don’t want to, OK, find someone to work with who does, and collaborate. Or do the prototyping, dump state, and hand off.
