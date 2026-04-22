Release a new version of Béilí by committing staged changes, updating the changelog, tagging, and pushing to GitHub.

## Steps

1. **Determine the new version**
   - Read the current version from `addon/config.yaml`
   - Suggest the next patch version (e.g. `1.3.4` → `1.3.5`) and ask the user to confirm or provide a different version

2. **Summarise staged changes**
   - Run `git diff --cached` to review what is staged
   - If nothing is staged, run `git status` and ask the user which files to stage

3. **Update CHANGELOG.md**
   - Add a new section at the top of the existing version group (or start a new group if the minor version changed) for the new version
   - Write concise, user-facing bullet points describing the staged changes — no internal chore/bump commits
   - Show the user the proposed changelog entry and ask them to confirm or edit it before continuing

4. **Bump the version**
   - Update the `version` field in `addon/config.yaml` to the new version

5. **Commit**
   - Stage `CHANGELOG.md` and `addon/config.yaml` alongside whatever the user had staged
   - Commit with message: `chore: bump version to <version>`

6. **Tag**
   - Create an annotated git tag: `git tag -a v<version> -m "v<version>"`

7. **Push**
   - Push the commit: `git push origin main`
   - Push the tag: `git push origin v<version>`
   - Confirm success to the user
