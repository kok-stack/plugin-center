package server

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	v1 "github.com/kok-stack/kok/api/v1"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/api/errors"
	v13 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"text/template"
)

func NewCommand() (*cobra.Command, context.Context, context.CancelFunc) {
	cancel, cancelFunc := context.WithCancel(context.TODO())
	config := &ApplicationConfig{}
	command := &cobra.Command{
		Use:   ``,
		Short: ``,
		Long:  ``,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			go func() {
				c := make(chan os.Signal, 1)
				signal.Notify(c, os.Kill)
				<-c
				cancelFunc()
			}()

			err := viper.Unmarshal(config)
			return err
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("%+v", config)

			if err := startServer(cancel, config); err != nil {
				return err
			}

			<-cancel.Done()
			return nil
		},
	}
	//command.AddCommand()
	viper.AutomaticEnv()
	viper.AddConfigPath(`.`)
	command.PersistentFlags().String("PluginDir", "plugins", "plugin dir path")
	command.PersistentFlags().String("Port", "8088", "port")
	command.PersistentFlags().Bool("Debug", true, "debug mode")
	err := viper.BindPFlags(command.PersistentFlags())
	if err != nil {
		panic(err.Error())
	}
	err = viper.BindPFlags(command.Flags())
	if err != nil {
		panic(err.Error())
	}
	viper.SetDefault("PluginDir", "plugin")

	return command, cancel, cancelFunc
}

func startServer(ctx context.Context, config *ApplicationConfig) error {
	err := initTemplateMaps(config)
	if err != nil {
		return err
	}
	r, s, err := initClient(ctx, config)
	engine := gin.Default()
	if !config.Debug {
		gin.SetMode(gin.ReleaseMode)
	}
	engine.Any("/download/:namespace/:name/:dir/:filename", func(ctx *gin.Context) {
		ns := ctx.Param("namespace")
		name := ctx.Param("name")
		dir := ctx.Param("dir")
		filename := ctx.Param("filename")

		cls := &v1.Cluster{}
		err = r.Get().Namespace(ns).Resource("clusters").Name(name).Do().Into(cls)
		if err != nil {
			if errors.IsNotFound(err) {
				ctx.Writer.WriteString(fmt.Sprintf("未找到集群,传入的集群名称%s可能存在错误", name))
			}
			panic(err)
		}

		t, ok := version2Addons[cls.Spec.ClusterVersion][dir]
		if !ok {
			if _, err := ctx.Writer.WriteString("未找到文件模板,传入的dir可能存在错误"); err != nil {
				panic(err)
			}
		}
		if err = t.ExecuteTemplate(ctx.Writer, filename, cls); err != nil {
			panic(err)
		}
		ctx.Writer.Flush()
	})
	engine.Any("/meta/:namespace/:name/ca/:filename", func(ctx *gin.Context) {
		getMeta(ctx, r, s, "ca")
	})

	engine.Any("/meta/:namespace/:name/nodeconfig/:filename", func(ctx *gin.Context) {
		getMeta(ctx, r, s, "nodeconfig")
	})

	return engine.Run(fmt.Sprintf(":%s", config.Port))
}

func initClient(ctx context.Context, config *ApplicationConfig) (*rest.RESTClient, *kubernetes.Clientset, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	clientConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, nil, err
	}
	newForConfig, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, nil, err
	}
	clientConfig.APIPath = "/apis"
	clientConfig.GroupVersion = &schema.GroupVersion{
		Group:   "cluster.kok.tanx",
		Version: "v1",
	}
	clientConfig.NegotiatedSerializer = scheme.Codecs
	forConfig, err := rest.RESTClientFor(clientConfig)
	if err != nil {
		return nil, nil, err
	}
	return forConfig, newForConfig, nil
}

var version2Addons = map[string]map[string]*template.Template{}
var defaultTemplateFuncs = template.FuncMap{"join": strings.Join}

func getMeta(ctx *gin.Context, r *rest.RESTClient, s *kubernetes.Clientset, dir string) {
	ns := ctx.Param("namespace")
	name := ctx.Param("name")
	filename := ctx.Param("filename")

	cls := &v1.Cluster{}
	err := r.Get().Namespace(ns).Resource("clusters").Name(name).Do().Into(cls)
	if err != nil {
		if errors.IsNotFound(err) {
			ctx.Writer.WriteString(fmt.Sprintf("未找到集群,传入的集群名称%s可能存在错误", name))
		}
		panic(err)
	}

	sourceName := ""
	switch dir {
	case "ca":
		sourceName = cls.Status.Init.CaPkiName
	case "nodeconfig":
		sourceName = cls.Status.Init.NodeConfigName
	}
	ca, err := s.CoreV1().Secrets(ns).Get(sourceName, v13.GetOptions{})
	if err != nil {
		panic(err)
	}
	b, ok := ca.Data[filename]
	if !ok {
		if _, err := ctx.Writer.WriteString("未找到元数据,传入的filename可能存在错误"); err != nil {
			panic(err)
		}
	}

	if _, err := ctx.Writer.Write(b); err != nil {
		panic(err)
	}
	ctx.Writer.Flush()
}

func initTemplateMaps(config *ApplicationConfig) error {
	dir, err := ioutil.ReadDir(config.PluginDir)
	if err != nil {
		return err
	}
	for _, sub := range dir {
		if !sub.IsDir() {
			continue
		}
		join := filepath.Join(config.PluginDir, sub.Name())
		subDir, err := ioutil.ReadDir(join)
		if err != nil {
			return err
		}
		m := map[string]*template.Template{}
		for _, info := range subDir {
			if !info.IsDir() {
				continue
			}
			name := filepath.Join(join, info.Name())
			t, err := template.New(name).Funcs(defaultTemplateFuncs).ParseGlob(name + "/*")
			if err != nil {
				return err
			}
			m[info.Name()] = t
		}

		version2Addons[sub.Name()] = m
	}
	return nil
}

type ApplicationConfig struct {
	PluginDir string
	Port      string
	Debug     bool
}
