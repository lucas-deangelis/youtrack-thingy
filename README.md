# Youtrack thingy

Add the link of your merge request to a youtrack ticket. If you're on branch `feat/ABC-956`, it'll look for the ticket `ABC-956` and add the link to the merge request on the comments of the ticket.

## Setup

You'll need Go installed on your machine.

Clone the repository.
Rename `.env.example` to `.env`, and fill it with a gitlab token, a youtrack token and your youtrack URL (example: `https://example.youtrack.cloud`).
Compile the project (`go build`).
Put it somewhere in your path (I use `.local/bin`).

## Usage

In a git repo, on branch `feat/ABC-956` that has an open merge request to `develop` or something:

```sh
$ youtrack-thingy
```
