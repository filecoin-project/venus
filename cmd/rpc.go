package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/filecoin-project/venus/app/paths"
	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/filecoin-project/venus/venus-shared/api"
	v1 "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"golang.org/x/xerrors"
)

var rpcCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interactive JsonPRC shell",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("method", true, false, "method name"),
		cmds.StringArg("params", false, false, "method params"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		repoDir, _ := req.Options[OptionRepoDir].(string)
		repoDir, err := paths.GetRepoPath(repoDir)
		if err != nil {
			return err
		}
		addr, err := repo.APIAddrFromRepoPath(repoDir)
		if err != nil {
			return err
		}
		token, err := repo.APITokenFromRepoPath(repoDir)
		if err != nil {
			return err
		}

		ai := api.NewAPIInfo(addr, token)
		addr, err = ai.DialArgs(api.VerString(v1.MajorVersion))
		if err != nil {
			return err
		}

		u, err := url.Parse(addr)
		if err != nil {
			return xerrors.Errorf("parsing api URL: %w", err)
		}

		switch u.Scheme {
		case "ws":
			u.Scheme = "http"
		case "wss":
			u.Scheme = "https"
		}

		addr = u.String()

		if len(req.Arguments) < 1 {
			return re.Emit("method name is required.")
		}
		methodName := req.Arguments[0]
		params := "[]"
		if len(req.Arguments) > 1 {
			params = req.Arguments[1]
		}

		res, err := send(addr, methodName, params, ai.AuthHeader())
		if err != nil {
			return err
		}

		return printOneString(re, res)
	},
}

func send(addr, method, params string, headers http.Header) (string, error) {
	jreq, err := json.Marshal(struct {
		Jsonrpc string          `json:"jsonrpc"`
		ID      int             `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}{
		Jsonrpc: "2.0",
		Method:  "Filecoin." + method,
		Params:  json.RawMessage(params),
		ID:      0,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", addr, bytes.NewReader(jreq))
	if err != nil {
		return "", err
	}
	req.Header = headers
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	rb, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if err := resp.Body.Close(); err != nil {
		return "", err
	}

	return string(rb), nil
}
