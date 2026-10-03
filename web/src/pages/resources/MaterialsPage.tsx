import type { CurrentUser } from '../../types/starline';
import { ContentResourcesPage } from './ContentResourcesPage';
export default function MaterialsPage({ user, courseId, syncLessonId, packageId, onClearFilter }: { user?: CurrentUser; courseId?: string; syncLessonId?: string; packageId?: string; onClearFilter?: () => void }) { return <ContentResourcesPage kind="materials" user={user} courseId={courseId} syncLessonId={syncLessonId} packageId={packageId} onClearFilter={onClearFilter} />; }
