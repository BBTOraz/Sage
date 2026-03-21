package agent

import "github.com/cloudwego/eino/components/tool"

func (a *Application) DocTools() []tool.BaseTool {
	out := make([]tool.BaseTool, len(a.docTools))
	copy(out, a.docTools)
	return out
}
