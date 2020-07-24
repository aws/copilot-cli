## Copilot Website
Copilot's website is built with [hugo](https://gohugo.io/) and the [docsy](https://www.docsy.dev/) theme.

### Pre-requisites
1. Make sure that you have installed the [hugo](https://gohugo.io/getting-started/installing/) binary.
2. Pull down the docsy theme. The docsy theme submodule is listed under the `.gitmodules` file.
   ```bash
   $ cd site/
   $ git submodule update --init --recursive
   ```
3. Install the [CSS processing libraries](https://www.docsy.dev/docs/getting-started/#install-postcss) required by docsy:
   ```bash
   $ cd site/
   $ npm install
   ```

### Releasing the latest docs to GitHub pages
Once we are ready to release a new version of the docs, you can run `make build-docs`.
Alternatively, you can:
```bash
$ cd site/
$ hugo
$ cd ..
```
This will update the documentation under the `docs/` directory. Afterwards, create a new PR with the changes:
```bash
$ git add docs/
$ git commit -m "docs: update website for copilot vX.Y.Z"
$ git push <remote> docs
```

### Developing locally

From the root of the repository, run `make start-docs`.  
Alternatively, you can:
```bash
$ cd site/
$ hugo server -D
```
Then you should be able to access the website at [http://localhost:1313/copilot-cli/](http://localhost:1313/copilot-cli/).

#### Adding new content
Most of the time, we'll need to just add new page under our documentation.  
Navigate to `./content/docs/` and add a new page by following the format of other markdown files.

#### Styling content

Docsy integrates by default with [bootstrap4](https://getbootstrap.com/docs/4.0/getting-started/introduction/), so we
can leverage any of the classes available there.

If you'd like to override any class that docsy itself generates, add the scss file under `assets/scss/`.
You can find which files are available under `themes/docsy/assets/scss`.
