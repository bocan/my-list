# On My List

A very small list application. One web page, one CSV file.

## Use

- Type a line of text in the box at the top and press Enter to add an entry.
- Click or tap the text of a tile to edit it. Right-click also works. Enter
  saves, Escape cancels.
- Drag a tile to change the order. On a touch screen, press and hold, then
  move.
- Click the cross on a tile to delete it, then confirm. With a mouse, the
  cross shows when you point at the tile. On a touch screen, it always shows.

## Categories

- An entry that starts with `> ` is a category: `> Work` shows as **Work** in
  red. Add more marks for a category in a category: `> > Projects`.
- An entry belongs to the nearest category above it, and shows indented.
  Entries above the first category are in no category.
- Drag an entry onto a category to put it in that category.
- Drag a category to move it together with all that is in it.
- A new entry goes to the top of the list. A new category goes to the bottom.
- To change the depth of a category, click its text and edit the marks.
- When you delete a category, its entries stay and go to the category above.

## Run

```sh
make run          # local, data in ./data, http://localhost:8080
make docker-run   # Docker, data in the named volume on-my-list-data
make test
```

To keep the data in a host directory:

```sh
docker run --rm -p 8080:8080 -v "$PWD/data:/data" on-my-list
```

The container runs as user 65532. On Linux, that user must have write access
to the host directory.

## Storage

All data is in `mylist.csv` in the data directory. The file mirrors the page:
one row for each tile, one column with the text, rows in the same order as
the tiles. There is no header and no ID column. A category row keeps its
marks (`> Work`). The indent is not stored: it comes from the row order.

You can edit the file by hand. The server reads it again when it changes, and
the page shows the change on the next reload.

A write that makes the file larger than the limit is refused, and the file
does not change.

## Configuration

| Variable    | Default                         | Function                    |
| ----------- | ------------------------------- | --------------------------- |
| `PORT`      | `8080`                          | HTTP port                   |
| `DATA_DIR`  | `data` (`/data` in the image)   | Directory for `mylist.csv`  |
| `MAX_BYTES` | `150000`                        | Maximum size of the file    |

There is no login. Do not put this on a public network without protection.

## License

Apache License 2.0. See [LICENSE](LICENSE).
