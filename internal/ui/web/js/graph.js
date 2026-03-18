import { nodes, links, nodesMap, linksMap, state, seamState } from './state.js';
import { isNodeVisible, getColor } from './ui.js';

let svg, g, zoom, simulation;
let width, height;
let registeredHandlers = {};

export function initGraph(handleNodeClick, handleNodeMouseOver, handleNodeMouseOut, handleNodeDoubleClick, handleNodeContextMenu) {
    registeredHandlers = {
        handleNodeClick,
        handleNodeMouseOver,
        handleNodeMouseOut,
        handleNodeDoubleClick,
        handleNodeContextMenu
    };

    width = document.getElementById('graph-container').clientWidth || 1200;
    height = document.getElementById('graph-container').clientHeight || 800;

    zoom = d3.zoom().on("zoom", function (event) {
        const transform = event.transform;
        g.attr("transform", transform);
        
        const shouldBeVisible = transform.k >= 0.6;
        if (shouldBeVisible !== state.labelsVisible) {
            state.labelsVisible = shouldBeVisible;
            g.selectAll('.node-label').style('opacity', shouldBeVisible ? 1 : 0);
        }
    });

    svg = d3.select('#graph-container')
        .append('svg')
        .attr('width', '100%')
        .attr('height', '100%')
        .style('cursor', 'grab')
        .call(zoom)
        .on("dblclick.zoom", null); 

    g = svg.append('g');

    svg.append("defs").append("marker")
        .attr("id", "arrowhead")
        .attr("viewBox", "0 -5 10 10")
        .attr("refX", 25)
        .attr("refY", 0)
        .attr("orient", "auto")
        .attr("markerWidth", 6)
        .attr("markerHeight", 6)
        .attr("xoverflow", "visible")
        .append("svg:path")
        .attr("d", "M 0,-5 L 10 ,0 L 0,5")
        .attr("fill", "#999")
        .style("stroke", "none");

    simulation = d3.forceSimulation()
        .force("link", d3.forceLink().id(d => d.id).distance(250))
        .force("charge", d3.forceManyBody().strength(-1000))
        .force("x", d3.forceX(width / 2).strength(0.05))
        .force("y", d3.forceY(height / 2).strength(0.05))
        .force("collision", d3.forceCollide().radius(45));

    simulation.on("tick", () => {
        g.selectAll(".link")
            .attr("x1", d => d.source.x)
            .attr("y1", d => d.source.y)
            .attr("x2", d => d.target.x)
            .attr("y2", d => d.target.y);

        g.selectAll(".node-group")
            .attr("transform", d => `translate(${d.x},${d.y})`);
    });

    return { svg, g, zoom, simulation };
}

export function getGraphComponents() {
    return { svg, g, zoom, simulation };
}

function isSemanticSeam(nodeId) {
    if (!seamState.semanticSeams) return false;
    for (let i = 0; i < seamState.semanticSeams.length; i++) {
        if (seamState.semanticSeams[i].method_a === nodeId || seamState.semanticSeams[i].method_b === nodeId) return true;
    }
    return false;
}

export function updateGraph(newNodes, newLinks) {
    const newlyAddedIds = new Set();
    newNodes.forEach(n => {
        const id = n.id || n.Id;
        if (!id) return;
        
        if (!nodesMap.has(id)) {
            const props = n.properties || n.Properties || {};
            const normalizedNode = {
                id: id,
                name: n.name || props.name || id,
                properties: props,
                x: width / 2, // Default to center
                y: height / 2,
                ...n
            };
            nodesMap.set(id, normalizedNode);
            nodes.push(normalizedNode);
            newlyAddedIds.add(id);
        } else {
            // Update properties if they changed
            const existing = nodesMap.get(id);
            const props = n.properties || n.Properties || {};
            existing.properties = { ...existing.properties, ...props };
            if (n.name) existing.name = n.name;
        }
    });

    if (newLinks) {
        newLinks.forEach(l => {
            const sourceId = l.sourceId || l.source;
            const targetId = l.targetId || l.target;
            const linkId = `${sourceId}-${targetId}-${l.type}`;
            if (!linksMap.has(linkId)) {
                const link = { source: sourceId, target: targetId, type: l.type };
                linksMap.set(linkId, link);
                links.push(link);

                // If one of the endpoints was just added, try to seed its coordinates from the other endpoint if it exists
                if (newlyAddedIds.has(sourceId) && nodesMap.has(targetId) && !newlyAddedIds.has(targetId)) {
                    const sourceNode = nodesMap.get(sourceId);
                    const targetNode = nodesMap.get(targetId);
                    sourceNode.x = targetNode.x;
                    sourceNode.y = targetNode.y;
                } else if (newlyAddedIds.has(targetId) && nodesMap.has(sourceId) && !newlyAddedIds.has(sourceId)) {
                    const sourceNode = nodesMap.get(sourceId);
                    const targetNode = nodesMap.get(targetId);
                    targetNode.x = sourceNode.x;
                    targetNode.y = sourceNode.y;
                }
            }
        });
    }

    renderGraph();
}

export function renderGraph() {
    const {
        handleNodeClick,
        handleNodeMouseOver,
        handleNodeMouseOut,
        handleNodeDoubleClick,
        handleNodeContextMenu
    } = registeredHandlers;

    // Links
    let linkSelection = g.selectAll(".link")
        .data(links, d => `${d.source.id || d.source}-${d.target.id || d.target}-${d.type}`);

    linkSelection.exit().remove();

    const linkEnter = linkSelection.enter().append("line")
        .attr("class", "link")
        .attr("stroke", "#94a3b8")
        .attr("stroke-width", 2)
        .attr("stroke-opacity", 0.6)
        .attr("marker-end", "url(#arrowhead)");

    linkSelection = linkEnter.merge(linkSelection);

    linkSelection.transition().duration(500)
        .style("opacity", d => {
            const source = d.source.id ? d.source : nodesMap.get(d.source);
            const target = d.target.id ? d.target : nodesMap.get(d.target);
            return (isNodeVisible(source) && isNodeVisible(target)) ? 1 : 0.05;
        });

    // Nodes
    let nodeSelection = g.selectAll(".node-group")
        .data(nodes, d => d.id);

    nodeSelection.exit().remove();

    const nodeEnter = nodeSelection.enter()
        .append('g')
        .attr('class', 'node-group')
        .call(d3.drag()
            .on("start", dragstarted)
            .on("drag", dragged)
            .on("end", dragended));

    if (handleNodeClick) nodeEnter.on("click", handleNodeClick);
    if (handleNodeMouseOver) nodeEnter.on("mouseover", handleNodeMouseOver);
    if (handleNodeMouseOut) nodeEnter.on("mouseout", handleNodeMouseOut);
    if (handleNodeDoubleClick) nodeEnter.on("dblclick", handleNodeDoubleClick);
    if (handleNodeContextMenu) nodeEnter.on("contextmenu", handleNodeContextMenu);

    nodeEnter.append('circle')
        .attr('class', 'node')
        .attr('r', 20)
        .attr('fill', d => getColor(d))
        .attr('stroke', '#fff')
        .attr('stroke-width', 2);

    nodeEnter.append('text')
        .attr('class', 'node-label')
        .attr('dy', 30)
        .attr('text-anchor', 'middle')
        .style('opacity', state.labelsVisible ? 1 : 0)
        .text(d => d.name || d.id);

    nodeSelection = nodeEnter.merge(nodeSelection);
    
    nodeSelection.transition().duration(500)
        .style("opacity", d => isNodeVisible(d) ? 1 : 0.1)
        .style("pointer-events", d => isNodeVisible(d) ? "auto" : "none");

    nodeSelection.select('circle')
        .attr('class', d => {
            let cls = 'node';
            if (seamState.showPinchPoints && seamState.pinchPoints.has(d.id)) cls += ' pinch-point';
            if (seamState.showSemanticSeams && isSemanticSeam(d.id)) cls += ' semantic-seam';
            return cls;
        });

    simulation.nodes(nodes);
    simulation.force("link").links(links);
    simulation.alpha(1).restart();
}

function dragstarted(event, d) {
    if (!event.active) simulation.alphaTarget(0.3).restart();
    d.fx = d.x;
    d.fy = d.y;
}

function dragged(event, d) {
    d.fx = event.x;
    d.fy = event.y;
}

function dragended(event, d) {
    if (!event.active) simulation.alphaTarget(0);
    // Keep the node fixed where it was dragged
    d.fx = d.x;
    d.fy = d.y;
}
