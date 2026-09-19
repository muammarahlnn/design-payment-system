# plain-english

A portable skill that makes an agent write simple English for a reader whose
first language is not English, while keeping software engineering terms exact.

## Install in another project

Copy the whole folder into the target project:

```bash
cp -r .agents/skills/plain-english /path/to/other-project/.agents/skills/
```

Then link it so Claude Code finds it:

```bash
cd /path/to/other-project
mkdir -p .claude/skills
ln -s ../../.agents/skills/plain-english .claude/skills/plain-english
```

If the other project has no `.agents/` folder, copy the skill straight into
`.claude/skills/plain-english` instead. The skill is self-contained, so either
location works.

Restart your agent session after copying. A skill is only listed at session
start.

## Make it always on

A skill loads on demand. The agent reads the description and decides if it is
relevant. That works well for a task ("rewrite this README"). It works less
well for a writing style, because a style applies to every reply, not to one
task.

To keep it always on, add this to the project's `CLAUDE.md`:

```markdown
## Writing style

Write in simple English. The reader speaks English as a second language.
Follow the `plain-english` skill in `.claude/skills/plain-english/SKILL.md`
for every reply, document, and comment. Read it now if you have not.

The short version: short sentences, common words, active voice, and no
phrasal verbs ("start", not "spin up"). Keep all software engineering terms
exactly as they are: idempotent, outbox pattern, consumer group, DLQ.
```

`CLAUDE.md` is read at the start of every session, so this makes the rule
apply by default.

## Install for other agents

`.agents/skills/` is the shared location that many coding agents read, so most
of them pick the skill up with no extra step. For an agent with no skill
system, paste the body of `SKILL.md` into its custom-instructions setting.

## Files

- `SKILL.md` — the rules. This is the part the agent reads.
- `references/word-swaps.md` — longer lists of words to replace. The agent
  reads this only when it needs a specific replacement.
- `README.md` — this file. For humans, not for the agent.
