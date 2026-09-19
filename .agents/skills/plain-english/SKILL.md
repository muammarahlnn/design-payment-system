---
name: plain-english
description: "Write in simple, plain English for a reader whose first language is not English, while keeping software engineering terms exact. Use this skill for every piece of prose you produce in this project - chat replies, README files, documentation, code comments, commit messages, PR descriptions, error strings - even when the user does not ask for simple English. Triggers: simpler english, plain english, too complex, too fancy, hard to read, non-native speaker, write it simply, easier words. The goal is short sentences and common words, NOT simplified technical content: keep terms like idempotent, outbox pattern, consumer group, and DLQ exactly as they are."
version: 1.0.0
---

# Plain English

The reader of your output speaks English as a second language. They are a
skilled software engineer. These two facts pull in opposite directions, and
getting the balance right is the whole job of this skill.

Hard English costs them real effort. Every rare word, every long sentence, and
every phrasal verb makes them stop and decode. That effort buys nothing.

Technical terms cost them nothing. `idempotent`, `connection pool`, and
`at-least-once delivery` are the shared vocabulary of their job. They already
know these words. Replacing them with simpler words loses precision, makes the
text longer, and treats an expert like a beginner.

So: **simplify the English, keep the engineering.**

## The single highest-value rule: no phrasal verbs

A phrasal verb is a verb plus a preposition, where the meaning does not follow
from the two words. "Spin up" has nothing to do with spinning. "Wire up" has
nothing to do with wire.

Native speakers learn these by exposure and never notice them. For everyone
else, they are the hardest part of English, because a dictionary does not help.
You cannot work out the meaning from the parts.

Replace every phrasal verb with one plain verb.

| Do not write | Write |
|---|---|
| spin up a container | start a container |
| wire up the config | create the config, connect the config |
| tear down the stack | stop the stack, remove the stack |
| kick off the job | start the job |
| reach out to the gateway | call the gateway |
| dive into the logs | read the logs |
| point it at the README | run it on the README |
| figure out the cause | find the cause |
| set up the database | create the database |
| end up failing | fail |
| run into an error | get an error |
| look into the issue | investigate the issue |
| roll back the migration | revert the migration |
| carry out the check | do the check |
| turn out to be slow | is slow |

`references/word-swaps.md` has a longer list, including rare words and literary
phrases to avoid.

## The other rules

These come from the structural half of ASD-STE100, a controlled-English
standard. They are mechanical, so apply them without thinking hard.

- **Short sentences.** One idea per sentence. Aim for 20 words or fewer.
- **Active voice.** Name who does the thing. Write "the worker reads the
  message", not "the message is read".
- **Simple tenses.** Write "the job failed", not "the job has ended up failing".
- **No semicolons.** Split the sentence in two.
- **Common words.** Write "use", not "leverage". Write "about", not "regarding".
- **No noun stacks.** Three words maximum. Rewrite "payment worker retry policy
  config" as "the retry config for the payment worker".
- **No literary flourishes.** No "the sharper line is", no "that lands close
  to", no rhetorical questions, no clever framing. State the point.
- **Keep structure.** Tables, bullet lists, and code blocks help this reader far
  more than flowing prose. Use them freely.

## Keep the engineering terms

Never replace a technical term with a vague description. Vagueness is worse than
a hard word, because the reader cannot look it up.

**Correct:** "The outbox pattern gives at-least-once delivery."
**Wrong:** "The system tries to send the message again." (vague, and not the
same claim)

**Correct:** "Make the handler idempotent."
**Wrong:** "Make sure running the handler twice is safe." (longer, and loses
the name of the concept)

Keep words like: idempotent, outbox pattern, inbox pattern, consumer group,
partition, offset, DLQ, transaction, connection pool, migration, healthcheck,
backpressure, at-least-once delivery, exactly-once semantics, race condition,
deadlock, index, foreign key, goroutine, mutex.

If a term is new to the project, define it once in one short sentence. Then
reuse the same term every time. Do not rotate between synonyms: pick "consumer"
or "worker" and keep it.

## What this is not

This is not translation, and not baby talk. The reader is an engineer. Do not
explain what a database is. Do not add "(a type of storage)" after Postgres.

This is also not a rule to write less. Cutting words is not the goal. Removing
difficulty is the goal. A clear four-sentence explanation beats a dense
one-sentence one.

## Before and after

**Before:**
> Once you've spun up the stack, the relay will end up draining the outbox
> table; if it runs into a broker error it'll back off and retry, which is
> generally the behavior you'd want here.

Problems: three phrasal verbs, one semicolon, 34 words in one sentence, and a
vague hedge at the end.

**After:**
> Start the stack. The relay then drains the outbox table. If it gets a broker
> error, it retries with a backoff. This gives at-least-once delivery.

Four short sentences. No phrasal verbs. The terms `outbox table`, `backoff`, and
`at-least-once delivery` stay exactly as they were.

## Check your own output

Before you send a reply, scan it for these five things:

1. Phrasal verbs. This is the most common miss.
2. Sentences longer than 20 words.
3. Semicolons.
4. Passive voice with no named actor.
5. A technical term you replaced with a vague phrase. Put the term back.
