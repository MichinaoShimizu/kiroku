# User testing

Scenarios and criteria for checking whether a change to the view actually helps users reach their goals. If "Changing the view" in `docs/development.md` is the builder's checklist, this is verification from the user's side. In Claude Code, the `user-tester` agent (`.claude/agents/user-tester.md`) tests along this document. The same scenarios work when asking people to try it.

AI testing does not replace real users. Use it as a first check to catch what might be missed, and give priority to feedback from people who actually used it.

## Setup

```bash
sh tools/screenshots/run.sh --html /tmp/kiroku-test.html   # HTML with dummy data
```

Open it with Playwright and try it in the following 3 environments. Keep the default dark theme, and take screenshots in dark too.

- 1440x900
- 1000x800
- 390x844 (phone)

Judge only by what you can see, without reading the view's code. Read the code only to explain the cause of a problem you ran into.

## Users and scenarios

Each scenario has the user's goal and a success condition. No steps are given. As the user, proceed using only the look and wording of the view as clues.

| # | User | Goal | Success condition |
|---|---|---|---|
| 1 | Someone opening it for the first time | Learn what this view is and what they did last week | Within 30 seconds, can say "what this view is" and "the project they spent the most time on last week" |
| 2 | Someone writing a weekly report | Paste last week's work into a weekly report | Opens and checks the weekly report draft text, copies it, and it is ready to paste as is. Notices that session names need editing |
| 3 | Someone who wants to keep AI costs down | Decide on one thing to change from next week | Picks one action from "Worth a look" and can say in their own words what to do |
| 4 | Someone who wants to know whether something they tried worked | Check whether what they tried last week worked | Finds the metric related to what they changed, from the 8-week trend of a marked metric or from a metric opened via "Worth a look" or "?", reads the direction of change and can judge whether it worked (including when it dropped off "Worth a look", or why it can't be judged) |
| 5 | Someone hunting the cause of a bug | Find the session where the AI touched a particular file | Reaches the session and commit from search, and can read the prompt flow |
| 6 | Someone who was stopped by a usage limit | Learn when and during what work they hit the limit | Finds the time and session of the limit hit, and can read what to do from "?" |
| 7 | Someone looking back on one session | Review a session that went badly with an AI | Opens the details and can copy the review prompt |
| 8 | Someone viewing on a phone | Glance at this week while on the move | Nothing overflows sideways, and can read the key figures, "Worth a look" and the calendar |
| 9 | Someone using only the keyboard | Move between weeks, open session details and close them | Can do it without a mouse, and can see where they are |
| 10 | Someone who wants to look back on the year and share it (skip while Year in review is hidden) | Turn this year's usage into one image to post on social media | Opens "Year in review", understands how to read the chart and why they got their "Your light" name, checks what is and isn't in the image, then saves the PNG |

## Criteria

While going through the scenarios, note the following.

- **Information architecture**
  - Can you predict where the information you want is (findability)
  - Can you correctly guess the content from names and labels (labels)
  - Do important things come first and details later (order and hierarchy)
  - Can you follow "what happened → why it matters → what to do next → how to check"
- **Wording**
  - No jargon, and no inconsistent terms within the view
  - Is it clear that estimated numbers are estimates
  - Does it avoid passing judgment on good or bad
- **UI**
  - No overlaps, overflow or cut-off text
  - Is the contrast sufficient
  - Do clickable things look clickable
- **UX**
  - Can you predict what happens when you press something
  - Can you go back
  - No information that can only be read by hovering
  - Is the result of an action clear (such as having copied something)
- **Trust**
  - Are the basis and thresholds of numbers clear
  - No presentation that could mislead (e.g. comparing a mid-week value side by side with a full week's value)

## Report format

For each scenario, summarize the following.

- **Result**: achieved / achieved with difficulty / not achieved
- **Path taken**: what you looked at, what you pressed, where you got lost (as the user thinking aloud)
- **Problems found**
  - Severity: blocks use / confusing or misleading / minor
  - Location (screenshot path)
  - Why it is a problem
  - Suggested fix

Finally, list problems by severity, grouping those with the same cause. Briefly note what worked well too.
