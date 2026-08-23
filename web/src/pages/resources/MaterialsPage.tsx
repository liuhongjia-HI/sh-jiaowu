import type { CurrentUser } from '../../types/starline';
import { ContentResourcesPage } from './ContentResourcesPage';
export default function MaterialsPage({ user, courseId, packageId, onClearFilter }: { user?: CurrentUser; courseId?: string; packageId?: string; onClearFilter?: () => void }) { return <ContentResourcesPage kind="materials" user={user} courseId={courseId} packageId={packageId} onClearFilter={onClearFilter} />; }
