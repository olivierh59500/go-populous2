# Validation de la base jouable

Vérifications exécutées sur macOS ARM64, avec Go 1.27.1 et Ebitengine 2.9.11.
Elles concernent le code actuel ; aucune parité de simulation Amiga n'est revendiquée.

| Vérification | Résultat |
|---|---|
| `go test ./...` | Réussi |
| `go vet ./...` | Réussi |
| `go test -race ./internal/amiga ./internal/populous2` | Réussi |
| Fuzz ADF, deux workers | 36 440 entrées, aucun échec |
| Fuzz Hunk, deux workers | 340 040 entrées, aucun échec |
| Fuzz décompression, deux workers | 56 506 entrées, aucun échec |
| 26 ressources attendues | Présentes, décodées ; XOR validés pour toutes les ressources compressées |
| Quatre décors | 255 tuiles et 830 descripteurs de sprites par décor ; différences de bâtiments appliquées |
| Campagne | 200 enregistrements, 1 000 mondes, premier code `DOEGAC` |
| Sauvegarde | Reprise avec effet de feu et RNG : même état après 120 tours supplémentaires |
| Fenêtre Ebitengine | Lancement, progression de 80 tours et capture du seul tampon de l'application |

La capture de la partie est [screenshots/game.png](../screenshots/game.png).
Les changements de décor se vérifient notamment sur les mondes 0, 25, 30 et 35,
premiers représentants de chacun des quatre jeux de graphismes.

## Simulations de travail

`go run ./cmd/simcheck -world 0 -ticks 4800` a terminé sur élimination au tour
4595, avec les populations `[31160, 0]`, 105 pouvoirs lancés et cette empreinte :

```text
13fac5a9175373e7d2034303b7d04dcfc0a5c175adc0c96a455699ef2f8ede76
```

Une seconde exécution a produit exactement la même empreinte. Le monde 500 a
terminé au tour 480, avec les populations `[0, 1791]` et 14 pouvoirs lancés :

```text
75a3d523cdc64f116945c20539eae288cf6c87b2e9a97121ce38109a02f0868b
```

Ces résultats montrent des parties reproductibles et des éliminations dans le
moteur de travail. Ils ne démontrent ni un équilibrage identique ni une IA
équivalente à Populous II. Les limites sont détaillées dans [PORTAGE.md](PORTAGE.md).
