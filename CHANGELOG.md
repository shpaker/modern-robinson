# Changelog

## 1.0.0 (2026-09-08)


### Возможности

* **app:** boot screens, UI toggles, text tests ([477254b](https://github.com/shpaker/modern-robinson/commit/477254b2a2a69d16223b9485223632ee9018ad6a))
* **app:** full intro chain and cutscene support ([0bb021d](https://github.com/shpaker/modern-robinson/commit/0bb021dced701542a96c31a16ea2ab9a9e900f5a))
* **app:** player settings in config.yml and a script trace ([4b6623d](https://github.com/shpaker/modern-robinson/commit/4b6623ddf776c59783771ddece0a31cfad37a594))
* **app:** quest integration and original 640x480 scrolling viewport ([c627e0f](https://github.com/shpaker/modern-robinson/commit/c627e0fa36745ef6cc214ac848c14d03dca91519))
* **app:** save/load and the island map button ([a8e3e9b](https://github.com/shpaker/modern-robinson/commit/a8e3e9bf528d7836b422a9b4a89969077280e396))
* **app:** scene override and interiors without a background ([d98385d](https://github.com/shpaker/modern-robinson/commit/d98385df8a94072fbb977bbb1a7288b6314da1e8))
* **audio:** scene music and SetMusic ([4e0a911](https://github.com/shpaker/modern-robinson/commit/4e0a911da6465b7a143ca83b67b92d4b206441c2))
* **bar:** inventory panel, startup vars, all-scene indexing ([af9c05d](https://github.com/shpaker/modern-robinson/commit/af9c05dffd65e154de41052072e7f471bfb77639))
* **bar:** read InvMaskLT from the panel layout ([8817f2b](https://github.com/shpaker/modern-robinson/commit/8817f2ba29738151db5b2d31651f2a6a582f1398))
* **char:** authored walk cycles, idle chains, cutscene skip ([164dd71](https://github.com/shpaker/modern-robinson/commit/164dd716ca8e90823f3870349bbf289690df110b))
* **codec:** decode method 0x100 (raw DEFLATE) — 100% resource coverage; stage-0 RE complete ([7d7186a](https://github.com/shpaker/modern-robinson/commit/7d7186aa150b9dbb0296d45fa5fe45fb0b7bee24))
* **engine:** ShiftScreen pans the camera; loop is the caller's choice ([7307881](https://github.com/shpaker/modern-robinson/commit/73078815d09f17a02c695c3ad1e3b6e7880c8293))
* **frid:** Friday as the second protagonist ([43809da](https://github.com/shpaker/modern-robinson/commit/43809dab6dc1c0b3e768cbe135e597bc1124ee27))
* **game:** Go/Ebitengine remake — scenes, walking, transitions, debug overlay ([165db1a](https://github.com/shpaker/modern-robinson/commit/165db1ae164182092505920d5e768915a71d0bff))
* **minigame:** the bamboo organ ([d5d5cc1](https://github.com/shpaker/modern-robinson/commit/d5d5cc146270b3db89bb0c6cf6f0a28472ea1f1f))
* **minigame:** the translator puzzle, and drop the WASM target ([50a3a61](https://github.com/shpaker/modern-robinson/commit/50a3a616cd762929c993d2e9edf842bda6cd4001))
* **minigame:** translator and organ follow the original rules ([bb9b047](https://github.com/shpaker/modern-robinson/commit/bb9b0479fe30ada0b180cd983dfbd22909835ce4))
* **prototype:** pygame reference prototype ([7eb5bfe](https://github.com/shpaker/modern-robinson/commit/7eb5bfea71ffad6cce5647422a1584664b68aab3))
* **quest:** interpreter, game state, persistent world edits ([5d6a3f1](https://github.com/shpaker/modern-robinson/commit/5d6a3f13a5d5ea1a39b1847e1964238597ff9433))
* **release:** package archives and attach them to the release ([ea403e1](https://github.com/shpaker/modern-robinson/commit/ea403e1f8decccd471eb010abc03fabfa466e120))
* **scene:** animate FonScript objects with Z-order, channel audio, hotspot fix ([91ca799](https://github.com/shpaker/modern-robinson/commit/91ca7995c96a65af4a49c844ee03cbe35a7924ba))
* **scene:** approach and act on clicked objects ([f95d205](https://github.com/shpaker/modern-robinson/commit/f95d205113b8d6969b6334629c6a2a4a3f75e8c6))
* **scene:** initial object state from BEGIN.BGI ([b4333c1](https://github.com/shpaker/modern-robinson/commit/b4333c194fd4ebb8b4e19b0958af1b0e5466e77a))
* **scene:** leaving a location plays its departure script ([3990c24](https://github.com/shpaker/modern-robinson/commit/3990c24d4cc26a1a2af431773031b91f2e24f11b))
* **screens:** the loading screen covers the intro handover ([21eb912](https://github.com/shpaker/modern-robinson/commit/21eb9126f63c62dcf11f6b724cd007ce00cd26ba))
* **sound:** scene ambience, and a skip that goes quiet ([9cfbe1b](https://github.com/shpaker/modern-robinson/commit/9cfbe1b5f6078c187d96c059b4c824d5da349620))
* **tools:** Python unpacker and codecs for NGI resources ([229a366](https://github.com/shpaker/modern-robinson/commit/229a36676e106cd35f71ecae2dd27c610347afdb))
* **ui:** game texts, authored icons, character portrait ([b91753b](https://github.com/shpaker/modern-robinson/commit/b91753b09c2ad799ff516ee785e38c97f03c850c))
* **ui:** options menu, twelve save slots, palette fades ([69426d7](https://github.com/shpaker/modern-robinson/commit/69426d7657963a6c703d3e3b5f2e0250c9f394e1))


### Исправления

* **actions:** Aproach is a playback event, the movie waits out the walk ([4bfd9ec](https://github.com/shpaker/modern-robinson/commit/4bfd9ecc0fbd5c13151d9fdbf1c7e346ce4773da))
* **bar:** authored icons cover every item ([d33db00](https://github.com/shpaker/modern-robinson/commit/d33db005631985fdf11ed72fe5df5af641851c2d))
* **bar:** draw the map and save buttons, the mask and the lit arrows ([d2b3a89](https://github.com/shpaker/modern-robinson/commit/d2b3a89708fbf2c31bdf675b23b46ad94a9eda77))
* **char:** the standing hero follows the cursor with his head ([e75f8f7](https://github.com/shpaker/modern-robinson/commit/e75f8f7d2610392dbd39bbbbd7f46dbf1f61ce0c))
* **char:** walking is the authored cycle chain, not interpolation ([2e1cf84](https://github.com/shpaker/modern-robinson/commit/2e1cf845507586fbf5903bdc7301d2327fc6177d))
* **grid:** objects block cells, SetVert persists, arrows avoid diagonals ([6c53e01](https://github.com/shpaker/modern-robinson/commit/6c53e01da5bcfad3b4b184e76ea71e3d6dcce8a5))
* **intro:** keep the hero off the opening cutscene bridges ([c8f1e57](https://github.com/shpaker/modern-robinson/commit/c8f1e57e766953eb60634f73886c06e2ab73cf48))
* **intro:** the opening room cutscene plays again ([fda95a7](https://github.com/shpaker/modern-robinson/commit/fda95a7e7aa9e0bfb87fc5b6e067ab61a2d1a406))
* **minigame:** the balloon can land, the hut keeps its pieces ([30ca749](https://github.com/shpaker/modern-robinson/commit/30ca7495801b5323bdc1f26cee2da36c7fd2088d))
* **parser:** objects named like keywords no longer drop themselves ([7265440](https://github.com/shpaker/modern-robinson/commit/7265440beb9e9647d43f198dbbdccaa4c73c83c2))
* **parser:** read an object's cell by fields, and keep all its hit zones ([001e979](https://github.com/shpaker/modern-robinson/commit/001e97916a779c72d6c0e1b43f6b223367732196))
* **render:** entry and action movies use the script's own Shift ([da79bc2](https://github.com/shpaker/modern-robinson/commit/da79bc2f0bacfd41208e7d0c84184bcdabb4626c))
* **scene:** a script's arrival cell is used as given ([8f3f369](https://github.com/shpaker/modern-robinson/commit/8f3f3691e79a93965e3d2302b51ee69461f881fd))
* **scene:** equal-z objects draw over the hero; robust object Shift ([2eba29a](https://github.com/shpaker/modern-robinson/commit/2eba29a55670f178ef04a1ddd250cee1223bdb7c))
* **scene:** exact engine placement for sprites and hit-zones ([b56d0d9](https://github.com/shpaker/modern-robinson/commit/b56d0d92644043265099edba3c2458e9dd029152))
* **scene:** exits come from the scene's own list, not from constants ([549d7c0](https://github.com/shpaker/modern-robinson/commit/549d7c07fe567f00940591fe0d97d31bb4a84b90))
* **scenes:** no island-discovery cutscene from a gated edge jump ([675a967](https://github.com/shpaker/modern-robinson/commit/675a96773077a2f32558b481e5a03bf1c510f13f))
* **script:** GoScene ends the frame and StartGame suspends it ([871e68b](https://github.com/shpaker/modern-robinson/commit/871e68b1212188e03c563d14979cabf8f1d2c140))
* **script:** honour the named Aproach object and the authored Z ([26db0d7](https://github.com/shpaker/modern-robinson/commit/26db0d7a08c669d3ee02b1eff78a1ca2b9a828b0))
* **state:** the active character and the item in hand are separate ([76da25c](https://github.com/shpaker/modern-robinson/commit/76da25c1d5646a4920757c4ac29a621dace7d070))
* **test:** resolve game resources by walking up from cwd ([3109fe8](https://github.com/shpaker/modern-robinson/commit/3109fe8a1d18d43751fe429bf0b97486cd40ab82))


### Рефакторинг

* make repository Go-focused — engine to root, drop pygame prototype ([3312201](https://github.com/shpaker/modern-robinson/commit/331220159b72507eaa6fbe351b37d1f662a70316))


### Документация

* bring the documentation up to date ([fce7432](https://github.com/shpaker/modern-robinson/commit/fce743203a32115e1efa3e3ca8fe0386380ad434))
* close the roadmap; Text strings were done in stage 4 ([c6be690](https://github.com/shpaker/modern-robinson/commit/c6be690494aef2d15fa1d630c18b4defd0f6a24d))
* exact scene geometry (cell/anchor/Shift, hit-zones, scroll) ([e49bea4](https://github.com/shpaker/modern-robinson/commit/e49bea46a795f8f1d22802461674866c1de2cc10))
* exits live in ObjectList; frame stops at GoScene and StartGame ([c800338](https://github.com/shpaker/modern-robinson/commit/c800338e10861bf0227864f440eeaa6d7559d8be))
* mark minigames, bar buttons, intro fix and packaging done ([fd7205a](https://github.com/shpaker/modern-robinson/commit/fd7205a782d2a86627d49bd1ff8afd9fc7e86342))
* mark stage-3 progress (bar, startup vars, scene index) ([bc6034c](https://github.com/shpaker/modern-robinson/commit/bc6034cf0c5498673fa9fc417d9a1f6924775717))
* minigame container inventory, shared clips and sound bank ([368e1f5](https://github.com/shpaker/modern-robinson/commit/368e1f5f6669d630847f293553fabb423d561f2b))
* minigame rules and constants from MINIGAME.DLL ([af68d90](https://github.com/shpaker/modern-robinson/commit/af68d907bf5aa39920ab411e9b4a12815cdc98c1))
* NGI engine format reverse-engineering notes ([022713d](https://github.com/shpaker/modern-robinson/commit/022713d0e4c0568bd35a063c95dc8fed5321b8d8))
* render todo.md as GitHub-flavored Markdown checklist ([1c520f0](https://github.com/shpaker/modern-robinson/commit/1c520f00490199a645380a7e8d8475c519d1ce99))
* roadmap reflects the finished engine work ([b29f270](https://github.com/shpaker/modern-robinson/commit/b29f270975fd0982bde02953470578f25415831a))
* the real BAR sprite map ([d0783be](https://github.com/shpaker/modern-robinson/commit/d0783bed14f34630fc171dabc5592230eb5cc982))
* third-party notices, shipped with release archives ([c465a52](https://github.com/shpaker/modern-robinson/commit/c465a521715f9de380bb17d436dd3bb8160855ef))
* walk model, idle head poses, exit scripts and action naming ([00fbb11](https://github.com/shpaker/modern-robinson/commit/00fbb11ec74988c5b30a213fc2a75a8a0f342f1c))


### Тесты

* **app:** cover the rules the engine work uncovered ([1ad9d48](https://github.com/shpaker/modern-robinson/commit/1ad9d48ddd69841fd45f3f27b2d48b6a7cc03239))
* **app:** golden file pins where every scene puts things ([3174546](https://github.com/shpaker/modern-robinson/commit/3174546c7cb994aa77bedeb0052b79b6e7d55514))
* **quest:** validate the quest from the data, and fix the resume order ([52a66ca](https://github.com/shpaker/modern-robinson/commit/52a66caa49df64332abb959302f07e878564a9e4))
