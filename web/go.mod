// This file only stops the Go tool from treating web/ (and the Go files some npm
// packages ship in node_modules) as part of the reefcare module: `./...` skips nested
// modules. There is no Go code here.
module reefcare/web

go 1.27.3
