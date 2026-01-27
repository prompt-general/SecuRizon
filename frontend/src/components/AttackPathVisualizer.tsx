import React, { useRef, useEffect } from 'react';
import * as d3 from 'd3';
import { AttackPath, PathNode } from '../types/attackpath';

interface AttackPathVisualizerProps {
  paths: AttackPath[];
  width?: number;
  height?: number;
  interactive?: boolean;
  onPathSelect?: (path: AttackPath) => void;
  onNodeSelect?: (node: PathNode) => void;
}

const AttackPathVisualizer: React.FC<AttackPathVisualizerProps> = ({
  paths,
  width = 800,
  height = 400,
  interactive = true,
  onPathSelect,
  onNodeSelect,
}) => {
  const svgRef = useRef<SVGSVGElement>(null);
  const tooltipRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!svgRef.current || paths.length === 0) return;

    const svg = d3.select(svgRef.current);
    svg.selectAll('*').remove(); // Clear previous render

    // Set up scales
    const xScale = d3.scaleLinear().domain([0, 100]).range([50, width - 50]);
    const yScale = d3.scaleLinear().domain([0, paths.length]).range([50, height - 50]);

    // Draw paths
    paths.forEach((path, pathIndex) => {
      const pathGroup = svg.append('g').attr('class', 'path-group');

      // Draw edges first (so they appear behind nodes)
      for (let i = 0; i < path.path.length - 1; i++) {
        const sourceNode = path.path[i];
        const targetNode = path.path[i + 1];

        const sourceX = xScale((i / (path.path.length - 1)) * 100);
        const sourceY = yScale(pathIndex);
        const targetX = xScale(((i + 1) / (path.path.length - 1)) * 100);
        const targetY = yScale(pathIndex);

        // Edge line
        pathGroup
          .append('line')
          .attr('x1', sourceX)
          .attr('y1', sourceY)
          .attr('x2', targetX)
          .attr('y2', targetY)
          .attr('stroke', getEdgeColor(sourceNode, targetNode))
          .attr('stroke-width', 2)
          .attr('stroke-dasharray', i === 0 ? 'none' : '5,5');

        // Edge arrow
        pathGroup
          .append('polygon')
          .attr('points', getArrowPoints(targetX, targetY, sourceX, sourceY))
          .attr('fill', getEdgeColor(sourceNode, targetNode));
      }

      // Draw nodes
      path.path.forEach((node, nodeIndex) => {
        const x = xScale((nodeIndex / (path.path.length - 1)) * 100);
        const y = yScale(pathIndex);

        // Node circle
        const circle = pathGroup
          .append('circle')
          .attr('cx', x)
          .attr('cy', y)
          .attr('r', 8)
          .attr('fill', getNodeColor(node))
          .attr('stroke', '#333')
          .attr('stroke-width', 1)
          .attr('class', 'node-circle')
          .style('cursor', interactive ? 'pointer' : 'default');

        if (interactive) {
          circle
            .on('mouseover', (event) => showTooltip(event, node))
            .on('mouseout', hideTooltip)
            .on('click', () => onNodeSelect?.(node));
        }

        // Node label
        pathGroup
          .append('text')
          .attr('x', x)
          .attr('y', y - 15)
          .attr('text-anchor', 'middle')
          .attr('font-size', '10px')
          .attr('fill', '#666')
          .text(getNodeLabel(node));
      });

      // Path risk label
      pathGroup
        .append('text')
        .attr('x', width - 60)
        .attr('y', yScale(pathIndex))
        .attr('text-anchor', 'end')
        .attr('font-size', '11px')
        .attr('fill', '#999')
        .text(`Risk: ${path.cumulative_risk.toFixed(1)}`);

      if (interactive) {
        pathGroup
          .style('cursor', 'pointer')
          .on('click', () => onPathSelect?.(path))
          .on('mouseover', function () {
            d3.select(this).selectAll('.node-circle').attr('r', 10);
          })
          .on('mouseout', function () {
            d3.select(this).selectAll('.node-circle').attr('r', 8);
          });
      }
    });

    // Legend
    const legend = svg
      .append('g')
      .attr('class', 'legend')
      .attr('transform', `translate(20, ${height - 30})`);

    const legendItems = [
      { label: 'Entry Point', color: '#ef4444' },
      { label: 'Compute', color: '#3b82f6' },
      { label: 'Identity', color: '#10b981' },
      { label: 'Data', color: '#8b5cf6' },
      { label: 'Target', color: '#f59e0b' },
    ];

    legendItems.forEach((item, i) => {
      legend
        .append('circle')
        .attr('cx', i * 100)
        .attr('cy', 0)
        .attr('r', 5)
        .attr('fill', item.color);

      legend
        .append('text')
        .attr('x', i * 100 + 10)
        .attr('y', 0)
        .attr('dy', '0.3em')
        .attr('font-size', '10px')
        .attr('fill', '#666')
        .text(item.label);
    });
  }, [paths, width, height, interactive]);

  const showTooltip = (event: MouseEvent, node: PathNode) => {
    if (!tooltipRef.current) return;

    const tooltip = d3.select(tooltipRef.current);
    tooltip
      .style('opacity', '1')
      .style('left', `${event.pageX + 10}px`)
      .style('top', `${event.pageY - 10}px`)
      .html(`
        <div class="p-2 text-sm">
          <strong>${node.type.toUpperCase()}</strong>
          <div class="text-gray-600">Risk: ${node.risk_score.toFixed(1)}</div>
          <div class="text-gray-600">Role: ${node.role}</div>
        </div>
      `);
  };

  const hideTooltip = () => {
    if (!tooltipRef.current) return;
    d3.select(tooltipRef.current).style('opacity', '0');
  };

  const getNodeColor = (node: PathNode): string => {
    switch (node.role) {
      case 'entry_point':
        return '#ef4444'; // red
      case 'target':
        return '#f59e0b'; // amber
      default:
        switch (node.type) {
          case 'compute':
            return '#3b82f6'; // blue
          case 'identity':
            return '#10b981'; // emerald
          case 'data':
            return '#8b5cf6'; // violet
          default:
            return '#6b7280'; // gray
        }
    }
  };

  const getEdgeColor = (source: PathNode, target: PathNode): string => {
    // Color edges based on the risk level of the connection
    const avgRisk = (source.risk_score + target.risk_score) / 2;
    if (avgRisk >= 70) return '#ef4444'; // red
    if (avgRisk >= 40) return '#f59e0b'; // amber
    return '#6b7280'; // gray
  };

  const getNodeLabel = (node: PathNode): string => {
    if (node.role === 'entry_point') return 'ENTRY';
    if (node.role === 'target') return 'TARGET';
    return node.type.slice(0, 3).toUpperCase();
  };

  const getArrowPoints = (x: number, y: number, sourceX: number, sourceY: number): string => {
    const angle = Math.atan2(y - sourceY, x - sourceX);
    const size = 6;
    const points = [
      [x, y],
      [x - size * Math.cos(angle - Math.PI / 6), y - size * Math.sin(angle - Math.PI / 6)],
      [x - size * Math.cos(angle + Math.PI / 6), y - size * Math.sin(angle + Math.PI / 6)],
    ];
    return points.map((p) => p.join(',')).join(' ');
  };

  return (
    <div className="relative">
      <svg
        ref={svgRef}
        width={width}
        height={height}
        className="border border-gray-200 rounded-lg bg-white"
      />
      <div
        ref={tooltipRef}
        className="fixed z-10 bg-white border border-gray-300 rounded shadow-lg pointer-events-none opacity-0 transition-opacity"
        style={{ minWidth: '150px' }}
      />
    </div>
  );
};

export default AttackPathVisualizer;