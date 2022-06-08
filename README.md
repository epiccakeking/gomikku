# Gomikku

Reimplementation of [Komikku](https://gitlab.com/valos/Komikku)'s reader in Go using Gio UI.
This is just a prototype to experiment with Gio UI, it does not modify Komikku's database so it shouldn't break anything if something goes wrong, but run at your own risk.

# Running
This depends on Gio UI and go-sqlite3, so you will need their dependencies to run this.
Use -datapath to specify the path to Komikku's data folder if not using Komikku through Flatpak.