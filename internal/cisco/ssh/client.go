package ssh

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/purpose-robot/planet-express/internal/cisco"
	"github.com/scrapli/scrapligo/v2/cli"
	"github.com/scrapli/scrapligo/v2/options"
	"github.com/sirikothe/gotextfsm"
)

type Client struct {
	conn *cli.Cli
}

type Config struct {
	Port             int
	Timeout          time.Duration
	TCPDialTimeout   time.Duration
	OperationTimeout time.Duration
}

func (c *Client) Open(ctx context.Context) error {
	_, err := c.conn.Open(ctx)
	if err != nil {
		return fmt.Errorf("%w: %w", cisco.ErrDiscovererConnectionFailed, err)
	}

	return nil
}

func (c *Client) Close(ctx context.Context) error {
	_, err := c.conn.Close(ctx)
	return err
}

func NewClient(host string, credential *cisco.Credential, config Config) (*Client, error) {
	opts := []options.Option{
		options.WithTransportSSH2(),
		options.WithPort(uint16(config.Port)),
		options.WithUsername(credential.Username()),
		options.WithPassword(credential.Password()),
		options.WithDefinitionFileOrName(cli.CiscoIosxe),
		options.WithOperationTimeout(config.OperationTimeout),
	}

	conn, err := cli.NewCli(host, opts...)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", cisco.ErrDiscovererInvalidOptions, err)
	}

	return &Client{conn: conn}, nil
}

func (c *Client) CollectInventory(ctx context.Context) (cisco.NetworkDeviceInventory, error) {
	softwareVersion, err := c.fetchString(
		ctx,
		"show version",
		"templates/cisco_ios_show_version.textfsm",
		"VERSION",
	)
	if err != nil {
		return cisco.NetworkDeviceInventory{}, err
	}

	return cisco.NetworkDeviceInventory{SoftwareVersion: softwareVersion}, nil
}

func (c *Client) fetchString(ctx context.Context, input, templatePath, key string) (string, error) {
	rows, err := c.parseMap(ctx, input, templatePath)
	if err != nil {
		return "", err
	}

	value, ok := rows[0][key].(string)
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%w: missing key %q", cisco.ErrDiscovererUnexpectedOutput, key)
	}

	return strings.TrimSpace(value), nil
}

func (c *Client) parseMap(ctx context.Context, input, templatePath string) ([]map[string]any, error) {
	resp, err := c.conn.SendInput(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", cisco.ErrDiscovererConnectionLost, err)
	}

	if resp.Failed() {
		return nil, fmt.Errorf("%w: %s", cisco.ErrDiscovererCommandFailed, strings.TrimSpace(resp.ResultsFailedIndicator))
	}

	template, err := templatesFS.ReadFile(templatePath)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", cisco.ErrDiscovererInvalidTemplate, err)
	}

	fsm := gotextfsm.TextFSM{}
	if err := fsm.ParseString(string(template)); err != nil {
		return nil, fmt.Errorf("%w: %w", cisco.ErrDiscovererInvalidTemplate, err)
	}

	output := gotextfsm.ParserOutput{}
	if err := output.ParseTextString(resp.Result(), fsm, true); err != nil {
		return nil, fmt.Errorf("%w: %w", cisco.ErrDiscovererUnexpectedOutput, err)
	}

	if len(output.Dict) == 0 {
		return nil, fmt.Errorf("%w: returned response map is empty", cisco.ErrDiscovererUnexpectedOutput)
	}

	return output.Dict, nil
}
