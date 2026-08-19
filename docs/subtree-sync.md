# Matching-engine subtree policy

`nogie-dev/matching-engine` is the source of truth for the matching engine.
`services/matching-engine` in this repository is an integration mirror managed
by Git subtree.

## Normal flow

1. Develop and merge changes in `nogie-dev/matching-engine`.
2. Its `main` workflow dispatches `matching-engine-updated` to this repository.
3. This repository fetches `matching-engine/main`, runs `git subtree pull`, and
   opens a pull request for review.
4. Merge the generated sync pull request into this repository's `main`.

Do not develop directly under `services/matching-engine` in this repository.
If a parent-first emergency change is unavoidable, export it explicitly with
`git subtree push` and open the source-repository pull request before merging
the parent change.

## Required secrets

- In `nogie-dev/matching-engine`, add `EXCHANGE_LAB_DISPATCH_TOKEN`. It needs
  permission to dispatch an event to `nogie-dev/exchange-lab`.
- In `nogie-dev/exchange-lab`, add `MATCHING_ENGINE_SYNC_TOKEN`. It needs
  read access to `nogie-dev/matching-engine`.

The exchange-lab workflow uses its own `GITHUB_TOKEN` to push a temporary
branch and open the sync pull request. Repository settings must allow Actions
to create pull requests.

The sync can also be started manually from the `Sync matching-engine subtree`
workflow when needed.
