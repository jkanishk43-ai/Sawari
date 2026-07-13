import { NextRequest, NextResponse } from 'next/server';

const BACKEND_URL = process.env.BACKEND_URL || 'http://localhost:8080';

export async function GET(request: NextRequest) {
  const searchParams = request.nextUrl.searchParams;

  const from = searchParams.get('from');
  const to = searchParams.get('to');
  const datetime = searchParams.get('datetime');
  const ac = searchParams.get('ac');
  const saheli = searchParams.get('saheli');
  const night = searchParams.get('night');

  if (!from || !to) {
    return NextResponse.json(
      { error: 'Missing required parameters: from, to' },
      { status: 400 }
    );
  }

  try {
    const params = new URLSearchParams({ from, to });
    if (datetime) params.set('datetime', datetime);
    if (ac) params.set('ac', ac);
    if (saheli) params.set('saheli', saheli);
    if (night) params.set('night', night);

    const response = await fetch(
      `${BACKEND_URL}/api/compare?${params.toString()}`,
      {
        headers: {
          'Content-Type': 'application/json',
        },
        signal: AbortSignal.timeout,
      }
    );

    if (!response.ok) {
      throw new Error(`Backend responded with status ${response.status}`);
    }

    const data = await response.json();
    return NextResponse.json(data);
  } catch (error) {
    console.error('Compare API error:', error);

    // Return mock data for demo when backend is unavailable
    return NextResponse.json(getMockCompareResponse(from, to), {
      headers: {
        'X-Data-Source': 'mock',
      },
    });
  }
}

function getMockCompareResponse(from: string, to: string) {
  return {
    success: true,
    dataSource: 'mock',
    route: { from, to },
    options: [
      {
        id: 'dtc-bus-104',
        provider: 'DTC Bus',
        mode: 'bus',
        fare: 15,
        currency: 'INR',
        duration: 45,
        eta: 10,
        badge: 'cheapest',
        features: ['AC Available', 'No Saheli'],
        deeplink: 'https://www.dtcbus.co.in',
      },
      {
        id: 'metro-blue',
        provider: 'DMRC Metro',
        mode: 'metro',
        fare: 30,
        currency: 'INR',
        duration: 35,
        eta: 5,
        badge: 'fastest',
        features: ['AC', 'WiFi', 'Saheli Coach'],
        deeplink: 'https://www.delhimetrorail.com',
      },
      {
        id: 'uber-go',
        provider: 'Uber Go',
        mode: 'cab',
        fare: 150,
        currency: 'INR',
        duration: 28,
        eta: 4,
        badge: 'smart',
        features: ['AC', '4 Seats', 'Contactless'],
        deeplink: 'uber://?action=setPickup&pickup=my_location',
      },
    ],
  };
}
