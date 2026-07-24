package annualsummary

import (
	"fmt"
	"strings"
)

func renderMarkdown(
	result Result,
	periodStart string,
	periodEnd string,
	evidenceCount int,
) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "# %s\n\n", result.Title)
	fmt.Fprintf(
		&builder,
		"> 总结周期：%s 至 %s  \n> 目标篇幅：约 %d 字  \n> 已确认依据：%d 条\n\n",
		periodStart,
		periodEnd,
		result.TargetWordCount,
		evidenceCount,
	)
	builder.WriteString("## 篇幅与章节大纲\n\n")
	for index, section := range result.Sections {
		fmt.Fprintf(
			&builder,
			"### %d. %s（约 %d 字）\n\n",
			index+1,
			section.Heading,
			section.WordCount,
		)
		for _, item := range section.Outline {
			fmt.Fprintf(&builder, "- %s\n", item)
		}
		fmt.Fprintf(
			&builder,
			"\n证据：%s\n\n",
			strings.Join(section.EvidenceIDs, "、"),
		)
	}

	builder.WriteString("## 可核实成果\n\n")
	if len(result.VerifiedResults) == 0 {
		builder.WriteString("- 当前证据不足以形成可核实成果，请补充个人分工或成果材料。\n")
	} else {
		for _, item := range result.VerifiedResults {
			fmt.Fprintf(
				&builder,
				"- %s（证据：%s）\n",
				item.Statement,
				strings.Join(item.EvidenceIDs, "、"),
			)
		}
	}

	builder.WriteString("\n## 信息不足 / 待补充\n\n")
	if len(result.MissingInformation) == 0 {
		builder.WriteString("- 暂无额外待补充项。\n")
	} else {
		for _, item := range result.MissingInformation {
			fmt.Fprintf(&builder, "- %s\n", item)
		}
	}
	return strings.TrimSpace(builder.String()) + "\n"
}
