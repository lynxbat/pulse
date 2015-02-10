package control

import (
	"crypto/rsa"
	"errors"

	"github.com/intelsdilabs/gomit"

	"github.com/intelsdilabs/pulse/control/plugin"
	"github.com/intelsdilabs/pulse/core/control_event"
)

// control private key (RSA private key)
// control public key (RSA public key)
// Plugin token = token generated by plugin and passed to control
// Session token = plugin seed encrypted by control private key, verified by plugin using control public key
//

type executablePlugins []plugin.ExecutablePlugin

type pluginControl struct {
	// TODO, going to need coordination on changing of these
	RunningPlugins executablePlugins
	Started        bool
	// loadRequestsChan chan LoadedPlugin

	controlPrivKey *rsa.PrivateKey
	controlPubKey  *rsa.PublicKey
	eventManager   *gomit.EventController
	subscriptions  *subscriptions
	pluginManager  ManagesPlugins
}

type ManagesPlugins interface {
	LoadPlugin(string) (*loadedPlugin, error)
	UnloadPlugin(CatalogedPlugin) error
	LoadedPlugins() *loadedPlugins
}

// TODO Update to newPluginControl
func Control() *pluginControl {
	c := new(pluginControl)
	c.eventManager = new(gomit.EventController)

	c.subscriptions = new(subscriptions)
	c.subscriptions.Init()

	c.pluginManager = newPluginManager()

	// c.loadRequestsChan = make(chan LoadedPlugin)
	// privatekey, err := rsa.GenerateKey(rand.Reader, 4096)

	// if err != nil {
	// 	panic(err)
	// }

	// // Future use for securing.
	// c.controlPrivKey = privatekey
	// c.controlPubKey = &privatekey.PublicKey

	return c
}

// Begin handling load, unload, and inventory
func (p *pluginControl) Start() {
	// begin controlling

	// Start load handler. We only start one to keep load requests handled in
	// a linear fashion for now as this is a low priority.
	// go p.HandleLoadRequests()

	// Start pluginManager when pluginControl starts
	p.Started = true
}

func (p *pluginControl) Stop() {
	// close(p.loadRequestsChan)
	p.Started = false
}

// Load is the public method to load a plugin into
// the LoadedPlugins array and issue an event when
// successful.
func (p *pluginControl) Load(path string) error {
	if !p.Started {
		return errors.New("Must start Controller before calling Load()")
	}

	if _, err := p.pluginManager.LoadPlugin(path); err != nil {
		return err
	}

	// defer sending event
	event := new(control_event.LoadPluginEvent)
	defer p.eventManager.Emit(event)
	return nil
}

func (p *pluginControl) Unload(pl CatalogedPlugin) error {
	err := p.pluginManager.UnloadPlugin(pl)
	if err != nil {
		return err
	}

	event := new(control_event.UnloadPluginEvent)
	defer p.eventManager.Emit(event)
	return nil
}

func (p *pluginControl) SwapPlugins(inPath string, out CatalogedPlugin) error {

	lp, err := p.pluginManager.LoadPlugin(inPath)
	if err != nil {
		return err
	}

	err = p.pluginManager.UnloadPlugin(out)
	if err != nil {
		err2 := p.pluginManager.UnloadPlugin(lp)
		if err2 != nil {
			return errors.New("failed to rollback after error" + err2.Error() + " -- " + err.Error())
		}
		return err
	}

	event := new(control_event.SwapPluginsEvent)
	defer p.eventManager.Emit(event)

	return nil
}

func (p *pluginControl) generateArgs() plugin.Arg {
	a := plugin.Arg{
		ControlPubKey: p.controlPubKey,
		PluginLogPath: "/tmp/pulse-test-plugin.log",
	}
	return a
}

// subscribes a metric
func (p *pluginControl) SubscribeMetric(metric []string) {
	key := getMetricKey(metric)
	p.subscriptions.Subscribe(key)
	e := &control_event.MetricSubscriptionEvent{
		MetricNamespace: metric,
	}
	p.eventManager.Emit(e)
}

// unsubscribes a metric
func (p *pluginControl) UnsubscribeMetric(metric []string) {
	key := getMetricKey(metric)
	err := p.subscriptions.Unsubscribe(key)
	if err != nil {
		// panic because if a metric falls below 0, something bad has happened
		panic(err.Error())
	}
	e := &control_event.MetricUnsubscriptionEvent{
		MetricNamespace: metric,
	}
	p.eventManager.Emit(e)
}

// the public interface for a plugin
// this should be the contract for
// how mgmt modules know a plugin
type CatalogedPlugin interface {
	Name() string
	Version() int
	TypeName() string
	Status() string
	LoadedTimestamp() int64
}

// the collection of cataloged plugins used
// by mgmt modules
type PluginCatalog []CatalogedPlugin

// returns a copy of the plugin catalog
func (p *pluginControl) PluginCatalog() PluginCatalog {
	table := p.pluginManager.LoadedPlugins().Table()
	pc := make([]CatalogedPlugin, len(table))
	for i, lp := range table {
		pc[i] = lp
	}
	return pc
}
