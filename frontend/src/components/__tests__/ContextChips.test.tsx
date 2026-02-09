import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import ContextChips from '../ContextChips';
import type { ContextEntity } from '../../types';

describe('ContextChips', () => {
  const entities: ContextEntity[] = [
    { type: 'datasource', id: 'prom-1', display_name: 'Prometheus', metadata: { ds_type: 'prometheus' } },
    { type: 'metric', id: 'up', display_name: 'up' },
  ];

  it('renders nothing when entities list is empty', () => {
    const { container } = render(<ContextChips entities={[]} onRemove={vi.fn()} />);
    expect(container.firstChild).toBeNull();
  });

  it('renders a chip per entity', () => {
    render(<ContextChips entities={entities} onRemove={vi.fn()} />);
    expect(screen.getByText('Prometheus')).toBeInTheDocument();
    expect(screen.getByText('up')).toBeInTheDocument();
  });

  it('calls onRemove when the X button is clicked', async () => {
    const onRemove = vi.fn();
    render(<ContextChips entities={entities} onRemove={onRemove} />);

    const removeBtn = screen.getByLabelText('Remove Prometheus');
    await userEvent.click(removeBtn);
    expect(onRemove).toHaveBeenCalledWith(entities[0]);
  });

  it('renders all entity type icons without errors', () => {
    const all: ContextEntity[] = [
      { type: 'datasource', id: 'ds-1', display_name: 'DS' },
      { type: 'dashboard', id: 'dash-1', display_name: 'Dash' },
      { type: 'metric', id: 'm-1', display_name: 'Metric' },
      { type: 'label', id: 'l-1', display_name: 'Label' },
    ];
    render(<ContextChips entities={all} onRemove={vi.fn()} />);
    expect(screen.getByText('DS')).toBeInTheDocument();
    expect(screen.getByText('Dash')).toBeInTheDocument();
    expect(screen.getByText('Metric')).toBeInTheDocument();
    expect(screen.getByText('Label')).toBeInTheDocument();
  });
});
