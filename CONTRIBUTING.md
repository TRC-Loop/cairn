# Contributing

First of all, **thank you** for taking the time to contribute to **cairn**!

This document includes rules and standards you must follow. Read this before opening issues or PRs[^1]

## Code of Conduct

Cairn is a small project maintained in spare time. The rules are short because they have to be remembered, not looked up.
Be respectful. Don't be rude to people asking questions, filing their first issue, or making mistakes. If you'd be embarrassed saying it face-to-face, don't type it.

Assume good faith. When something someone wrote could be read two ways, pick the charitable reading. Non-native English speakers sometimes sound blunt when they're being neutral. New contributors miss things because they haven't read the docs yet, not because they're disrespecting you. Ask before you assume.
Disagree on ideas, not people. "This approach has a race condition" is fine. "You clearly don't understand concurrency" is not. Code can be wrong. People shouldn't be attacked for writing it.

Zero tolerance for harassment or discrimination. Targeting someone for their race, gender, sexuality, religion, nationality, or disability, threatening, or doxxing anyone gets you banned from the project. No warnings, no debate.

Report problems to [me@arne.sh](mailto:me@arne.sh). Reports are handled privately.

### Language

Many people that contribute aren't native english speakers (myself included), so make language easy to understand and use Footnote References to explain complicated words or abbreviations (`[^1]`). To see an example of Footnote References, look at the source code of this document. Example Footnote[^2]

## AI Usage

You can use AI if you want to. It would be nice of you to disclose that you're using AI in your commit message or PR[^1] description. If you're using a tool like Claude Code or Codex you can use this prompt: `When commiting, add yourself as co-author.`. You don't have to disclose this, but community members might appreciate this.

When using AI, please don't leave AI-looking comments everywhere. If a comment makes sense, keep it, but comments everywhere bloat the code.

There's only one important thing: autonomous AI Agents are not allowed. A Human has to create the PR[^1]. **Spam PRs[^1] by AI are prohibited.**

## Ways to Contribute

- Bug reports: [Issue Tab](https://github.com/TRC-Loop/cairn/issues)
- Bug fixes
- Translations
- Docs improvements (Typos, ...)
- Feature proposals. Discuss first, then implement.
- PR review (especially welcome from returning contributors)

## Before Opening an Issue or PR[^1]

For **small changes** (Typo fixes, one-line doc fixes, obvious small bug fixes), **create the PR directly**.

For **bigger changes** (new feature, refactor, behavior change), **create issue first**, then the PR.

> [!CAUTION]
> Surprise large PRs without prior discussion may be closed without merge

## Developer Certificate of Origin (DCO)

### Full DCO text

The full, unmodified text of the Developer Certificate of Origin 1.1 is in [DCO.md](./DCO.md). When you sign off a commit, that's what you're attesting to.

### Why

Two things matter when you send code to Cairn:

1. You actually wrote it, or you're allowed to submit it. Code copied from a GPL project, code owned by your employer, or code an AI generated from copyrighted training data can't be accepted unless the permissions check out.
2. You're fine with it being AGPL-3.0 from that point on. Once it's merged it stays under that license.

The DCO is how you confirm both, in one line, per commit.

### How to sign off

Set your name and email in git, once:

```bash
git config --global user.name "John S."
git config --global user.email "john@example.com"
```

Then pass `-s` when you commit:

```bash
git commit -s -m "fix: handle nil response in HTTP check"
```

That adds a line at the bottom of your commit message:

```
Signed-off-by: John S. <john@example.com>
```

The DCO bot checks every PR. If a commit is missing the sign-off, it blocks the merge until you fix it.

> [!TIP]
> If using Claude Code or Codex, use a prompt like `Add \`-s\` to git commits.`

#### Info you provide in your Sign-offs

Cairn doesn't require your full legal name. Any of these work:

- `John Smith`
- `John S.`
- `John`

What doesn't work:

- GitHub username only, with no real name part (`cooluser42`)
- Pure pseudonym (`DarkWizard`)

The email matters more than the name. It has to be real and reachable, because that's what we use if there's ever a question about a contribution. GitHub's noreply addresses (`12345+user@users.noreply.github.com`) are fine.

> [!NOTE]
> Once you commit with a name and email, they're in git history forever. Rewriting history after the fact breaks everyone else's clones. Pick something you're okay with being public long-term. If you are a minor, ask your parents what they are okay with you providing.

#### Fixing a missed sign-off

If you forgot `-s` on your last commit:

```bash
git commit --amend -s --no-edit
git push --force-with-lease
```

For multiple commits in your branch, rebase and sign off all of them:

```bash
git rebase HEAD~N --signoff
git push --force-with-lease
```

Replace `N` with the number of commits you need to fix.

## Licensing

Cairn is licensed under **AGPL-3.0-or-later**. By opening a pull request, you agree your contributions are licensed under the same terms. There's no separate form to sign, the DCO covers it.

New source files should start with an SPDX header so the license is machine-readable:

```go
// SPDX-License-Identifier: AGPL-3.0-or-later
```

```svelte
<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
```

If your employer has an IP assignment agreement[^3] in your contract (common in tech jobs), check that you're allowed to contribute before you send a PR. That's on you, not us.

## Principles

It's **highly recommended** to read [PRINCIPLES.md](./PRINCIPLES.md). It includes things like:

- Accessibilty Requirements
- Design (Colors, Fonts, Guidelines)
- What cairn is and isn't
- Privacy defaults

Read this especially when making changes to the User Interface/UX[^4].

## Getting Help

Different questions belong in different places:

- **Bugs with a reproduction**: [Issues](https://github.com/TRC-Loop/cairn/issues)
- **Questions, ideas, "is this a bug?"**: [Discussions](https://github.com/TRC-Loop/cairn/discussions)
- **Security issues or private contact**: [me@arne.sh](mailto:me@arne.sh), not a public issue (see [SECURITY.md](./SECURITY.md))

Cairn is solo-maintained in spare time so replies can take a few days. Please be patient.

If you want to chat informally, I'm on Discord at [arne.sh/discord](https://arne.sh/discord). It's personal server, not Cairn-specific (But has cairn channels), and not the right place for bug reports.

[^1]: PR is short for *P*ull *R*equest

[^2]: This is an example footnote reference.

[^3]: An IP assignment agreement is a clause in your employment contract that gives your employer ownership of code you write, sometimes including code written on your own time or unrelated to your job. Most tech contracts have one. If yours does, check whether it covers open-source contributions before you send a PR.

[^4]: UX stands for *U*ser E*x*perience.
