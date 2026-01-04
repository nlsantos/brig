# devcontainer-jekyll

**A [devcontainer](https://containers.dev) configuration for working with Jekyll**

Utilizing [BretFisher/jekyll-serve](https://github.com/BretFisher/jekyll-serve)'s OCI images, it is composed of two services:

- a devcontainer service that runs `bash` and has [Jekyll](https://jekyllrb.com/) ready for generating pages
- a Jekyll server serving your files on port 4000

## Usage

You can copy the `devcontainer.json` and `compose.yml` file into your `.devcontainer` directory.

You could also use [`git-subrepo`](https://github.com/ingydotnet/git-subrepo) to link it to your codebase by running:

```bash
git subrepo clone https://github.com/nlsantos/devcontainer-jekyll .devcontainer
```

inside your Git repository. You can then use your preferred tooling to spin up the devcontainer.

Take a look at my project [`brig`](https://github.com/nlsantos/brig) if you don't already have one.

## License

In case _configuration_ can be copyrighted, these files are made available to the public under [0BSD](https://www.tldrlegal.com/license/bsd-0-clause-license).