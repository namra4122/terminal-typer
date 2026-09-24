# Terminal Typer roadmap

This directory keeps numbered implementation records and planning outlines. Plans `001`-`003` cover maintenance, runtime settings, and the existing TUI redesign spec. Plans `004`-`008` capture the five-phase terminal-native typing roadmap from the [Monkeytype Web Research conversation](chatgpt-conversation://6ab4a90c-db1c-83ee-83cc-a715450953cc). They are ordered backlogs for writing smaller implementation specs, not authorization to change the CLI contract or release the product.

| Phase | Plan | User outcome |
| --- | --- | --- |
| 1 | [004 - Typing engine and content](004-typing-engine-content.md) | Select real typing modes and focused content packs through one session model. |
| 2 | [005 - Metrics, history, and analytics](005-metrics-history-analytics.md) | Inspect detailed results, local history, trends, and personal bests. |
| 3 | [006 - TUI experience and visual polish](006-tui-experience.md) | Control the test interactively through a polished terminal interface. |
| 4 | [007 - Training, difficulty, and keyboards](007-training-difficulty-keyboards.md) | Practice weak spots and track performance by keyboard layout. |
| 5 | [008 - Presets, Funbox, and extensibility](008-presets-funbox-extensibility.md) | Save setups and extend local typing content and modifiers. |

```text
004 Engine and content
  -> 005 Metrics and local history
    -> 006 Interactive TUI
      -> 007 Adaptive training
        -> 008 Presets and extensibility
```

The order follows data dependencies. Analytics needs the session events and test metadata from `004`; live widgets and history screens need the metrics from `005`; adaptive practice needs that history; presets need stable configuration for the preceding features. Independent asset authoring and theme design can happen alongside implementation, but integration and compatibility checks follow this order.

Each bullet under "Spec candidates" is a proposed small, demonstrable spec. Before implementation, write its concrete behavior, data formats, migration and rollback rules, terminal interactions, and compatibility tests in a numbered plan. Keep existing flags, stdin and file modes, resource lookup, JSON and CSV output, and local-only storage working unless an explicitly approved plan changes that contract. Run `make verify` and `make smoke` for each code change. Update `README.md` and `man.md` together for public behavior changes; regenerate the manual through `make assets` when `man.md` changes. Do not edit generated `src/packed.go` directly.

The original conversation mentioned online races, leaderboards, cloud sync, accounts, a plugin API, and an API server only as optional ideas. They are outside this roadmap because the current product contract keeps user text and progress local.
