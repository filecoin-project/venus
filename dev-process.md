### Goals of the go-filecoin dev process

Based on [#915](https://github.com/filecoin-project/go-filecoin/issues/915) our goals are:

1. Openness -- organize work in the open. Make it easy to engage.
1. Agility -- react quickly to changing opportunities. Be able to quickly refocus dev resources on new priorities.
1. Efficiency -- eliminate impediments and don’t waste time. Ensure priority is clear. Make it easy to find what to do next. Empower saying "no" to distractions.
1. Predictability -- know roughly how long work will take. 

### Big picture

We keep a [Kanban board in ZenHub](https://app.zenhub.com/workspace/o/filecoin-project/go-filecoin/boards?repos=113219518,150652727,126356739,135468185) and drive it via a weekly [scrum](https://en.wikipedia.org/wiki/Scrum_(software_development)#Key_ideas)-like process. If someone believes a story is both well-scoped and a priority they place it in the ready Candidates column. The ready Candidates are reviewed during the weekly Storytime call where they are either promoted to Ready or sent back the Backlog. Devs pull stories out of the Ready queue and move them through In Progress, Review/AQ, and Closed columns. The Backlog holds a working set of stories that are getting close to being priorities. The Icebox holds stories for later.

Anyone may suggest a Candidate or take an unassigned issue from Ready. We generally promote Candidates to Ready based on consensus, though use your judgement: a new P0 crash bug might go directly to Ready.

### Well-scoped work

Only well-scoped work will be promoted to Ready. In order to be well-scoped the story should:

  - **include detail** enabling someone familiar with go-filecoin to get started. If you mark something [a good first issue for newbs](https://github.com/filecoin-project/go-filecoin/issues?q=is%3Aissue+is%3Aopen+label%3AE-good-first-issue), include more detail (and try to preserve those issues for newbs if you can).
  - **be [esimated](https://help.zenhub.com/support/solutions/articles/43000010347-estimate-work-using-story-points)** using the following scale:
     - _Small / 1_ - Low uncertainty, follows established patterns, limited in scope. No land mines.
     - _Medium / 5_ - Some uncertainty, moderate scope or breaking new ground. Probably contains some gotchas.
     - _Large / 13_ - High uncertainty and/or large scope, far-reaching, or hairy. Could easily have hidden gotchas that significantly increase scope or effort.
     - _Probably an epic / 34_ - Placeholder for signficant work we want to track but hasn't been fleshed out yet. It is unlikely we'd put a work unit this size into Ready.
  - **include acceptance criteria**, a specific set of things that must be true in order for the issue to be considered complete. It must be easy to determine if they are true.

### Developer responsibilities

Generally speaking:
- pick work from Ready. It will be roughly prioritized. Use your judgement about working on stuff not in Ready.
- assign an issue to yourself when you are working on it and move it through the appropriate In Progress, Reivew/QA and Closed columns.
- respect the acceptance criteria
- come to the Storytime meeting prepared to offer your input 
- if you think we should work on something, create a well-scoped issue and include it as a Candidate or in the Backlog if it's not yet a priority

Note this last bullet well: _writing up stories is everyone's responsibility_, we need to all get good at this. 

### Additional detail

- **ZenHub extension**: the [ZenHub Chrome extension](https://chrome.google.com/webstore/detail/zenhub-for-github/ogcgkffhplmphkaahpmffcafajaocjbd?hl=en-US) enables you to add zenhub-related fields (releases, estimates, etc) directly through github -- you'll see drop downs for them to the right
- **Releases**: if it's important that an issue be address by a particular release, be sure to tag it with that release
- **[Labels](https://github.com/filecoin-project/go-filecoin/labels)**: the most useful [labels](https://github.com/filecoin-project/go-filecoin/labels) are P0 (stop and help with this), E-help-wanted, and E-good-first-issue, please use them. Also needs-story-detail means something is under-scoped.
- **Working groups**: WGs are devs focused on a specific area, eg Proofs. A repo != WG; a WG is a workstream and may cross repos. All dev WGs should represent their work as esimated stories associated with releases on the board, though the process by which their work moves through the columns is up to them. The process above is for go-filecoin.

### Meetings

There is a dev-wide meeting on Tuesday used for announcements and discussion, a high-level prioritization meeting with Juan and/or whyrusleeping on Wednesday for WG captains and anyone interested, and the go-filecoin Storytime meeting on Thursday.
