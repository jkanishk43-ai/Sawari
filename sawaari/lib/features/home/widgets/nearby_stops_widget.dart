import 'package:flutter/material.dart';
import '../../../../core/theme/app_theme.dart';

class NearbyStopsWidget extends StatelessWidget {
  const NearbyStopsWidget({super.key});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 20),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                'Nearby Stops',
                style: Theme.of(context).textTheme.titleMedium?.copyWith(
                      fontWeight: FontWeight.w600,
                    ),
              ),
              TextButton(
                onPressed: () {},
                child: const Text('See all'),
              ),
            ],
          ),
          const SizedBox(height: 12),
          SizedBox(
            height: 100,
            child: ListView(
              scrollDirection: Axis.horizontal,
              children: [
                _StopCard(
                  name: 'Rajiv Chowk Metro',
                  distance: '0.3 km',
                  lines: ['Blue', 'Yellow'],
                  icon: Icons.subway,
                ),
                _StopCard(
                  name: 'Barakhamba Bus Stop',
                  distance: '0.5 km',
                  lines: ['DTC', 'Cluster'],
                  icon: Icons.directions_bus,
                ),
                _StopCard(
                  name: 'Mandi House Metro',
                  distance: '0.7 km',
                  lines: ['Blue', 'Violet'],
                  icon: Icons.subway,
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _StopCard extends StatelessWidget {
  final String name;
  final String distance;
  final List<String> lines;
  final IconData icon;

  const _StopCard({
    required this.name,
    required this.distance,
    required this.lines,
    required this.icon,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 180,
      margin: const EdgeInsets.only(right: 12),
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(16),
        boxShadow: [
          BoxShadow(
            color: Colors.black.withValues(alpha: 0.05),
            blurRadius: 10,
            offset: const Offset(0, 2),
          ),
        ],
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                padding: const EdgeInsets.all(8),
                decoration: BoxDecoration(
                  color: AppTheme.primaryColor.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Icon(
                  icon,
                  size: 16,
                  color: AppTheme.primaryColor,
                ),
              ),
              const Spacer(),
              Text(
                distance,
                style: Theme.of(context).textTheme.labelSmall?.copyWith(
                      color: AppTheme.textSecondary,
                    ),
              ),
            ],
          ),
          const Spacer(),
          Text(
            name,
            style: Theme.of(context).textTheme.labelMedium?.copyWith(
                  fontWeight: FontWeight.w600,
                ),
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
          const SizedBox(height: 4),
          Wrap(
            spacing: 4,
            children: lines.map((line) {
              return Container(
                padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                decoration: BoxDecoration(
                  color: AppTheme.primaryColor.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(4),
                ),
                child: Text(
                  line,
                  style: Theme.of(context).textTheme.labelSmall?.copyWith(
                        color: AppTheme.primaryColor,
                        fontSize: 10,
                      ),
                ),
              );
            }).toList(),
          ),
        ],
      ),
    );
  }
}
