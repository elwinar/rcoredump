# Release Process

## Update the changelog

Ensure the changelog is complete, and that redundant or unneeded lines are
removed, e.g. fixes for features introduced by an earlier commit but absent in
the previous version.

Add the next version header in the changelog, and the link to the release page.
Create the release draft on github and ensure that the link is correct.

Commit the file, and tag the commit with the new version.

## Ensure the readme is up to date

Especially command-line options, any assertion about behavior.

Update the screenshot by compiling the webapp (`make web`), the server (`make
rcoredumpd`), and taking a screenshot of the homepage with the standard dataset
(all the crashers run thrice in succession, so the screenshot exhibits the
pagination). Ensure the version displayed is the correct one (it should be
taken from the git tag when compiling the server.)

Amend with the screenshot and the readme. Update the tag.

## Push the release commit

And the tag.

## Build the compiled versions

`make release` will do it all alone, provided the commit and tag are pushed to
the github repository.

Upload the results in the release draft.

## Release the version

Ensure the _pre-release_ checkbox is ticked, and hit the _publish_ button.

## Update the milestone

Close the one for the published version. Create the next version's milestone
and choose the issues to tackle.
