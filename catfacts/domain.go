package catfacts

import (
	"context"
	"fmt"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes catfacts as a kit Domain: a driver that a multi-domain
// host (ant) enables with a single blank import,
//
//	import _ "github.com/tamnd/catfacts-cli/catfacts"
//
// exactly as a database/sql program enables a driver with `import _
// "github.com/lib/pq"`. The init below registers it; the host then dereferences
// catfacts:// URIs by routing to the operations Register installs. The same
// Domain also builds the standalone catfacts binary (see cli.NewApp), so the
// binary and a host share one source of truth.
func init() { kit.Register(Domain{}) }

// Domain is the catfacts driver. It carries no state; the per-run client is
// built by the factory Register hands kit.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against, and
// the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "catfacts",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "catfacts",
			Short:  "A command line for CatFact Ninja.",
			Long: `A command line for CatFact Ninja.

catfacts reads public catfact.ninja data over plain HTTPS, shapes it into
clean records, and prints output that pipes into the rest of your tools. No API
key, nothing to run alongside it.`,
			Site: Host,
			Repo: "https://github.com/tamnd/catfacts-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	// fact: fetch a single random cat fact.
	kit.Handle(app, kit.OpMeta{Name: "fact", Group: "read", Single: true,
		Summary: "Fetch a random cat fact", URIType: "fact", Resolver: true}, getFact)

	// facts: fetch multiple cat facts.
	kit.Handle(app, kit.OpMeta{Name: "facts", Group: "read", List: true,
		Summary: "List cat facts", URIType: "fact"}, getFacts)

	// breeds: fetch cat breeds.
	kit.Handle(app, kit.OpMeta{Name: "breeds", Group: "read", List: true,
		Summary: "List cat breeds", URIType: "breed"}, getBreeds)
}

// newClient builds the client from the host-resolved config, so a host and the
// standalone binary pace and identify themselves the same way.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := NewClient()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.HTTP.Timeout = cfg.Timeout
	}
	return c, nil
}

// --- inputs ---

type factInput struct {
	Client *Client `kit:"inject"`
}

type factsInput struct {
	Limit  int     `kit:"flag,inherit" help:"max facts" default:"10"`
	Client *Client `kit:"inject"`
}

type breedsInput struct {
	Limit  int     `kit:"flag,inherit" help:"max breeds" default:"20"`
	Client *Client `kit:"inject"`
}

// --- handlers ---

func getFact(ctx context.Context, in factInput, emit func(*Fact) error) error {
	f, err := in.Client.GetFact(ctx)
	if err != nil {
		return mapErr(err)
	}
	return emit(f)
}

func getFacts(ctx context.Context, in factsInput, emit func(*Fact) error) error {
	facts, err := in.Client.GetFacts(ctx, in.Limit)
	if err != nil {
		return mapErr(err)
	}
	for i := range facts {
		if err := emit(&facts[i]); err != nil {
			return err
		}
	}
	return nil
}

func getBreeds(ctx context.Context, in breedsInput, emit func(*Breed) error) error {
	breeds, err := in.Client.GetBreeds(ctx, in.Limit)
	if err != nil {
		return mapErr(err)
	}
	for i := range breeds {
		if err := emit(&breeds[i]); err != nil {
			return err
		}
	}
	return nil
}

// --- Resolver: the URI-native string functions, pure and network-free ---

// Classify turns any accepted input into the canonical (type, id).
// For catfact.ninja there is no meaningful id, so everything maps to "fact".
func (Domain) Classify(input string) (uriType, id string, err error) {
	if input == "" {
		return "", "", errs.Usage("empty catfacts reference")
	}
	return "fact", input, nil
}

// Locate is the inverse: the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "fact":
		return fmt.Sprintf("%s/fact", BaseURL), nil
	case "breed":
		return fmt.Sprintf("%s/breeds", BaseURL), nil
	default:
		return "", errs.Usage("catfacts has no resource type %q", uriType)
	}
}

// mapErr converts a library error into the kit error kind that carries the
// right exit code.
func mapErr(err error) error {
	return err
}
