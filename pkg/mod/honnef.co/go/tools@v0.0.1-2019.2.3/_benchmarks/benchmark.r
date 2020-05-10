library("ggplot2")
library("sitools")

kb <- function(x) { f2si(x * 1000) }

data <- read.table("bench", header=TRUE, sep=" ", colClasses=c("character", "character", "character", "numeric", "numeric"))

ggplot(data = data, aes(x = Version, y = Time)) +
    geom_point() +
    scale_x_discrete(limits=data$Version) +
    facet_wrap(~ Target)

ggplot(data = data, aes(x = Version, y = Memory)) +
    geom_point() +
    scale_x_discrete(limits=data$Version) +
    scale_y_continuous(labels = kb) +
    facet_wrap(~ Target)

